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
	data  []byte // capture data in host memory
	wrPtr uint64 // current host memory write pointer position
	// duration (in seconds) between two latency timestamp counter increments
	tickPeriodLatency float64
	caplen            int  // maximum per-packet capture length
	discard           bool // if true, captured data is discarded
}

// WriteToFile writes the captured data to an output file.
func (capture *Capture) WriteToFile(filename string) {
	err := ioutil.WriteFile(filename, capture.data[0:capture.wrPtr], 0644)
	if err != nil {
		Log(LOG_ERR, "Capture '%s': could not write to file", filename)
	}

	Log(LOG_DEBUG, "Capture '%s': wrote to file", filename)
}

// GetPackets returns a list of captured packets.
func (capture *Capture) GetPackets() CapturePackets {
	var pkts CapturePackets
	var posRd uint64

	for posRd < capture.wrPtr {
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
			// calculate latency in seconds
			latency = float64(meta&0xFFFFFF) * capture.tickPeriodLatency

			// subtract latency error induced by the MACs and PHYs of the
			// network tester itself
			latency -= float64(LATENCY_ERR_CORRECTION_CYCLES) / FREQ_SFP
		}

		// get packet's arrival-time (time since previous packet arrived, the
		// arrival-time value of the first packet is not meaningful)
		arrivalTime := float64((meta>>25)&0xFFFFFFF) / FREQ_SFP

		// get packet's wire length
		wirelen := int((meta >> 53) & 0x7FF)

		// determine capture length
		var caplen int
		if wirelen > capture.caplen {
			caplen = capture.caplen
		} else {
			caplen = wirelen
		}

		// create new CapturePacket struct
		pkt := CapturePacket{
			ArrivalTime: arrivalTime,
			HasLatency:  hasLatency,
			Latency:     latency,
			Wirelen:     wirelen,
		}

		// save packet data
		pkt.Data = make([]byte, caplen)
		copy(pkt.Data, capture.data[posRd+8:posRd+8+uint64(caplen)])

		// apennd CapturePacket struct to list
		pkts = append(pkts, pkt)

		// calculate position of next packet's meta data (each 8 byte meta data
		// word is followed by the capture data, which is aligned to 8 byte
		// boundaries)
		if caplen%8 == 0 {
			posRd += 8 + uint64(caplen)
		} else {
			posRd += 16 + uint64(caplen-caplen%8)
		}
	}

	return pkts
}

// GetSize returns the size of trace capture data in bytes.
func (capture *Capture) GetSize() uint64 {
	// size of captured data is equal to current write pointer position
	return capture.wrPtr
}

// getWriteSlice returns an empty byte slice of size 'size' to which capture
// data can be written to.
func (capture *Capture) getWriteSlice(size uint32) []byte {
	if capture.discard {
		// captured data shall be discarded. always write data to the same
		// (sub-) byte slice
		return capture.data[0:size]
	}

	// get slice
	wrSlice := capture.data[capture.wrPtr : capture.wrPtr+uint64(size)]

	// increment write pointer position
	capture.wrPtr += uint64(size)

	// return slice
	return wrSlice
}
