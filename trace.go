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
// Network trace.

package gofluent10g

import (
	"bufio"
	"io/ioutil"
	"os"
	"time"
)

// Trace is a struct representing a trace whose content should be replayed by
// the network tester.
type Trace struct {
	size     uint64 // size of the trace in bytes
	nRepeats int    // number of time the trace is replayed
	data     []byte // trace data

	fromFile bool // flag indicating whether the trace has been read from a file

	// number of packets in the trace. currently only set for synthetically
	// generated traces, not for traces read from a file
	nPackets int

	// duration of the trace. currently only set for synthetically generated
	// traces, not for traces read from a file
	duration time.Duration
}

// TraceCreateFromFile creates a trace instance for a trace specified by its
// filename. The function also expects a parameter specifying the number of
// times the trace shall be replayed.
func TraceCreateFromFile(filename string, nRepeats int) *Trace {
	// open the trace file
	traceFile, err := os.Open(filename)
	if err != nil {
		Log(LOG_ERR, "Trace '%s': could not open file", filename)
	}
	defer traceFile.Close()

	// get file info
	traceFileInfo, err := traceFile.Stat()
	if err != nil {
		Log(LOG_ERR, "Trace '%s': could not stat file", filename)
	}

	// get the file size
	traceFileSize := traceFileInfo.Size()

	// file size must always be a multiple of 64 bytes
	if traceFileSize%64 != 0 {
		Log(LOG_ERR, "Trace '%s': invalid file size (must be a multiple of "+
			"64 bytes)", filename)
	}

	// create a trace struct and store information
	trace := Trace{
		size:     uint64(traceFileSize),
		nRepeats: nRepeats,
		fromFile: true,
	}

	// allocate data slice memory
	trace.data = make([]byte, traceFileSize)

	// create file reader
	r := bufio.NewReader(traceFile)

	Log(LOG_DEBUG, "Trace '%s': reading file", filename)

	// file size is a multiple of 64 bytes, so read data in 64 byte chunks
	for i := int64(0); i < traceFileSize/64; i++ {
		_, err := r.Read(trace.data[i*64 : (i+1)*64])
		if err != nil {
			Log(LOG_ERR, "Trace '%s': could not read file", filename)
		}
	}

	Log(LOG_DEBUG, "Trace '%s': reading file done", filename)

	return &trace
}

// TraceCreateFromData creates a trace instance for a trace specified by its
// data in form of a byte slice. The function also expects parameters
// specifying the number of packets the trace includes, the duration and the
// number of times the trace shall be replayed.
func TraceCreateFromData(data []byte, nPackets int, duration time.Duration, nRepeats int) *Trace {
	// trace size must be a multiple of 64 bytes
	if len(data)%64 != 0 {
		Log(LOG_ERR, "Trace: invalid size (must be a multiple of 64 bytes)")
	}

	// create Trace
	trace := Trace{
		size:     uint64(len(data)),
		data:     data,
		fromFile: false,
		nPackets: nPackets,
		nRepeats: nRepeats,
		duration: duration,
	}
	return &trace
}

// WriteFile writes the trace data to an output file.
func (trace *Trace) WriteFile(filename string) {
	err := ioutil.WriteFile(filename, trace.data, 0644)
	if err != nil {
		Log(LOG_ERR, "Trace '%s': could not write file")
	}
}

// GetSize returns the size of the trace in bytes. If the trace is repeatedly
// replayed, the function returns the size of the actual trace data multiplied
// by the number of replays.
func (trace *Trace) GetSize() uint64 {
	return uint64(trace.nRepeats) * trace.size
}

// GetPacketCount returns the number of packets the trace includes. If the
// trace is repeatedly replayed, the number of packets is multiplied by the
// number of replays. The packet count currently cannot be obtained for packets
// that have been read from a file.
func (trace *Trace) GetPacketCount() int {
	if trace.fromFile {
		Log(LOG_ERR, "Cannot obtain packet count from trace that has been "+
			"read from file")
	}

	return trace.nPackets * trace.nRepeats
}

// GetDuration returns the trace duration. If the trace is repeatedly replayed,
// the duration is multiplied by the number of replays. The duration currently
// cannot be obtained for packets that have been read from a file.
func (trace *Trace) GetDuration() time.Duration {
	if trace.fromFile {
		Log(LOG_ERR, "Cannot obtain duration from trace that has been "+
			"read from file")
	}

	return trace.duration * time.Duration(trace.nRepeats)
}

// GetData returns the trace data. If the trace is repeatedly replayed, only
// the data for the first replay is returned.
func (trace *Trace) GetData() []byte {
	return trace.data
}

// read reads data from the input trace file. The function expects the address
// from which shall be read and the number of bytes that shall be read. In case
// a trace file is replayed multiple times, the address parameter may be larger
// than the trace file. This function handles the wrap-around.
func (trace *Trace) read(addr uint64, size uint32) []byte {
	// make sure the provided address is within the valid range
	if addr > uint64(trace.nRepeats)*trace.size {
		Log(LOG_ERR, "Trace read address exceeds trace size")
	}

	if addr/trace.size != (addr+uint64(size)-1)/trace.size {
		// file read extends across wrap-around memory boundary

		// number of bytes to read until end of file
		size1 := uint32(trace.size - addr%trace.size)

		// number of bytes remaining to be read from beginning of file
		size2 := size - size1

		// create empty slice
		data := make([]byte, size)

		// copy data to slice
		copy(data[0:size1],
			trace.data[addr%trace.size:addr%trace.size+uint64(size1)])
		copy(data[size1:], trace.data[0:size2])

		return data
	}
	// file read does not extend across wrap-around memory boundary
	return trace.data[addr%trace.size : addr%trace.size+uint64(size)]
}
