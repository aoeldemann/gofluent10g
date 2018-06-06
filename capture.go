// The MIT License
//
// Copyright (c) 2017-2018 by the author(s)
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
//
// Author(s):
//   - Andreas Oeldemann <andreas.oeldemann@tum.de>
//
// Description:
//
// Capture data.

package gofluent10g

import (
	"encoding/binary"
	"io/ioutil"
)

// Capture is a struct representing network data that is captured on a single
// network interface.
type Capture struct {
	data    []byte // capture data
	size    uint64 // size of captured data
	posWr   uint64 // current write pointer position
	discard bool   // discard captured data? useful for debugging
	// duration (in seconds) between latency two timestamp counter increments
	tickPeriodLatency float64
	maxLenCapture     uint64 // maximum per-packet capture length
}

// WriteToFile writes the captured data to an output file.
func (capture *Capture) WriteToFile(filename string) {
	if capture.discard {
		Log(LOG_ERR, "Capture '%s': data has been discarded", filename)
	}

	err := ioutil.WriteFile(filename, capture.data[0:capture.posWr], 0644)
	if err != nil {
		Log(LOG_ERR, "Capture '%s': could not write to file", filename)
	}

	Log(LOG_DEBUG, "Capture '%s': wrote to file", filename)
}

// GetPackets returns a list of captured packets.
func (capture *Capture) GetPackets() CapturePackets {
	if capture.discard {
		Log(LOG_ERR, "Capture: data has been discarded")
	}

	var pkts CapturePackets
	var posRd uint64

	for {
		// get 8 byte meta data word
		meta := binary.LittleEndian.Uint64(capture.data[posRd : posRd+8])

		if meta == 0xFFFFFFFFFFFFFFFF {
			// end of capture data
			break
		}

		// has a latency value been calculated for this packet?
		hasLatency := (meta>>24)&0x1 == 0x1

		// extract latency value, if present
		var latency float64
		if hasLatency {
			latency = float64(meta&0xFFFFFF) * capture.tickPeriodLatency
		}

		// get packet's arrival-time (time since previous packet arrived, the
		// arrival-time value of the first packet is not meaninful)
		arrivalTime := float64((meta>>25)&0xFFFFFFF) / FREQ_SFP

		// get packet's wire length
		lenWire := (meta >> 53) & 0x7FF

		// determine capture length
		var lenCapture uint64
		if lenWire > capture.maxLenCapture {
			lenCapture = capture.maxLenCapture
		} else {
			lenCapture = lenWire
		}

		// create new CapturePacket struct
		pkt := CapturePacket{
			ArrivalTime: arrivalTime,
			HasLatency:  hasLatency,
			Latency:     latency,
			LenWire:     uint(lenWire),
		}

		// save packet data
		pkt.Data = make([]byte, lenCapture)
		copy(pkt.Data, capture.data[posRd+8:posRd+8+lenCapture])

		// apennd CapturePacket struct to list
		pkts = append(pkts, pkt)

		// calculate position of next packet's meta data (each 8 byte meta data
		// word is followed by the capture data, which is aligned to 8 byte
		// boundaries)
		if lenCapture%8 == 0 {
			posRd += 8 + lenCapture
		} else {
			posRd += 16 + lenCapture - lenCapture%8
		}

		if posRd >= capture.posWr {
			// we are done!
			break
		}
	}

	return pkts
}

// GetSize returns the size of trace capture data in bytes. Even if data
// is discarded after fetching it from the hardware, the size represents
// the amount of data that has been fetched.
func (capture *Capture) GetSize() uint64 {
	return capture.size
}

// captureCreate creates and returns a capture instance. The amount of memory
// that is reserved for capturing is defined by the memSize parameter. The
// parameter tickPeriodLatency specifies the time (in seconds) that passes
// between two subsequent latency timestamp counter increments. The parameter
// maxLenCapture specifies the configured maximum per-packet capture length.
// If the parameter discard is set to true, the capture data will be fetched
// from the hardware, but will be discarded after it has been fetched (the
// parameter memSize will be ignored). This is helpful for debugging and
// network tester performance analysis without consuming huge amounts of memory
// for capture data.
func captureCreate(nt *NetworkTester, memSize uint64, tickPeriodLatency float64, maxLenCapture int, discard bool) *Capture {
	var data []byte
	if discard == false {
		// not discarding data, so reserve memory
		data = make([]byte, memSize)
	} else {
		// capture data will be discarded. Reserve enough memory to store
		// a single ring buffer read. This memory will be overwritten on
		// every read.
		data = make([]byte, RING_BUFF_RD_TRANSFER_SIZE_MIN)
	}

	return &Capture{
		data:              data,
		discard:           discard,
		tickPeriodLatency: tickPeriodLatency,
		maxLenCapture:     uint64(maxLenCapture),
	}
}

// getWriteSlice returns an empty byte slice of size 'size' to which capture
// data can be written to.
func (capture *Capture) getWriteSlice(size uint32) []byte {
	// get slice
	wrSlice := capture.data[capture.posWr : capture.posWr+uint64(size)]

	// if capture data is being discarded, the same slice is being written to
	// over and over again. if data is not discarded, increment the write
	// pointer on every write.
	if capture.discard == false {
		// increment write position
		capture.posWr += uint64(size)
	}

	// increment capture size
	capture.size += uint64(size)

	// return slice
	return wrSlice
}
