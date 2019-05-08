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
// This file implements the Generator struct, which configures and controls the
// trace replay hardware. The hardware is structed as follows:
//
//   --------------       ------       ---------       -----
//  | DRAM         |     | BRAM |     | Rate    |     | MAC |
//  | TX Ring Buff | --> | FIFO | --> | Control | --> |     |
//   --------------       ------       ---------       -----
//
// Trace data is copied from the host running the software to a TX ring buffer
// in the DRAM of the FPGA board. As soon as the hardware is triggered to read
// trace data from the ring buffer (by calling the start() function), it reads
// data from the ring buffer and copies it to a Block RAM FIFO. Data is
// continously refilled by the writeRingBuff() function. Hardware pauses reading
// data from the ring buffer as soon as the Block RAM FIFO becomes full. The
// Rate Control module enforces the inter-packet transmission times specified
// in the trace file. Only when the Rate Control module is activated (see
// generators.go), the packets are read from the BRAM FIFO and forwarded to
// the MAC for transmission on the link.

package gofluent10g

import (
	"fmt"
	"time"
)

// Generator is the struct providing methods for configurating the trace replay
// hardware cores. Each instance of the struct corresponds to one hardware core
// on one network interface. The struct additionally provides methods for
// transferring data from host to the TX ring buffers on the FPGA board.
type Generator struct {
	nt *NetworkTester
	id int

	trace *Trace // trace file assigned to this generator

	// number of trace files that have been transferred to hardware
	nBytesTransfered uint64

	// ring buffer memory address, size and write pointer position
	ringBuffAddr      uint64
	ringBuffAddrRange uint32 // ring buffer must never be larger than 4 Gbyte
	ringBuffWrPtr     uint32
}

// SetTrace assigns a trace file to the generator for replay.
func (gen *Generator) SetTrace(trace *Trace) {
	gen.trace = trace
}

// configHardware initializes the generator configuration and writes the
// configuration to the hardware.
func (gen *Generator) configHardware() {
	// nothing to do here if no trace is configured for this generator
	if gen.trace == nil {
		return
	}

	// calculate ring buffer size
	ringBuffSize := uint64(gen.ringBuffAddrRange) + 1

	// the ring buffer size must be larger than 16384 bytes
	if ringBuffSize <= 16384 {
		Log(LOG_ERR,
			"Generator %d: ring buffer size must be larger than 16384 bytes.",
			gen.id)
	}

	// the ring buffer size must be a multiple of 16384 bytes
	if ringBuffSize%16384 != 0 {
		Log(LOG_ERR,
			"Generator %d: ring buffer size must be a multiple of 16384 bytes.",
			gen.id)
	}

	// the ring buffer transfer size must be a multiple of 16384 bytes
	if RING_BUFF_WR_TRANSFER_SIZE_MAX%16384 != 0 {
		Log(LOG_ERR,
			"Generator %d: ring buffer transfer size must be a multiple of "+
				"16384 bytes.", gen.id)
	}

	// the ring buffer transfer size must be smaller than the size of ring
	// buffer
	if ringBuffSize <= RING_BUFF_WR_TRANSFER_SIZE_MAX {
		Log(LOG_ERR,
			"Generator %d: ring buffer transfer size must be smaller than "+
				"ring buffer size", gen.id)
	}

	// get pcie bar access module
	pcieBAR := gen.nt.pcieBAR

	// write ring buffer memory region start address and range
	pcieBAR.Write(ADDR_BASE_NT_GEN_REPLAY[gen.id]+
		CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_MEM_ADDR_HI, uint32(gen.ringBuffAddr>>32))
	pcieBAR.Write(ADDR_BASE_NT_GEN_REPLAY[gen.id]+
		CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_MEM_ADDR_LO, uint32(gen.ringBuffAddr&0xFFFFFFFF))
	pcieBAR.Write(ADDR_BASE_NT_GEN_REPLAY[gen.id]+
		CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_MEM_RANGE, gen.ringBuffAddrRange)

	// reset ring buffer write pointer
	gen.ringBuffWrPtr = 0x0
	pcieBAR.Write(ADDR_BASE_NT_GEN_REPLAY[gen.id]+
		CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_ADDR_WR, gen.ringBuffWrPtr)

	Log(LOG_DEBUG,
		"Generator %d: replay from ring buffer addr 0x%016x, range 0x%016x",
		gen.id, gen.ringBuffAddr, gen.ringBuffAddrRange)

	// no trace data has been transferred yet, set number of transferred bytes
	// to zero
	gen.nBytesTransfered = 0

	// get trace size
	traceSize := gen.trace.GetSize()

	// write trace size to hardware
	pcieBAR.Write(ADDR_BASE_NT_GEN_REPLAY[gen.id]+
		CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_TRACE_SIZE_HI, uint32(traceSize>>32))
	pcieBAR.Write(ADDR_BASE_NT_GEN_REPLAY[gen.id]+
		CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_TRACE_SIZE_LO, uint32(traceSize&0xFFFFFFFF))
}

// writeRingBuff writes trace data to the generator's TX ring buffer in the DRAM
// memory of the FPGA board. It returns the number of bytes that have been
// transferred.
func (gen *Generator) writeRingBuff() uint32 {
	if gen.trace == nil {
		// nothing to do here
		return 0
	}

	// get the trace size
	traceSize := gen.trace.GetSize()

	// calculate number of bytes that remain to be transfered
	traceSizeOutStanding := traceSize - gen.nBytesTransfered

	// outstanding number of bytes must never become negative
	if traceSizeOutStanding < 0 {
		Log(LOG_ERR, "Generator %d: ring buffer write failed", gen.id)
	}

	if traceSizeOutStanding == 0 {
		// trace has been completely written, we are done here
		return 0
	}

	// get ring buffer size
	ringBuffSize := uint64(gen.ringBuffAddrRange) + 1

	// get current write pointer value. the write pointer is an address offset.
	// the base address is the start of the ring buffer memory region.
	ringBuffWrPtr := gen.ringBuffWrPtr

	// calculate the memory size of the ring buffer from the current position
	// of the write pointer until the end
	ringBuffSizeEnd := ringBuffSize - uint64(ringBuffWrPtr)

	// calculate the number of bytes we will transfer
	var transferSize uint32
	if traceSizeOutStanding <= RING_BUFF_WR_TRANSFER_SIZE_MAX {
		transferSize = uint32(traceSizeOutStanding)
	} else {
		transferSize = RING_BUFF_WR_TRANSFER_SIZE_MAX
	}
	if ringBuffSizeEnd <= uint64(transferSize) {
		transferSize = uint32(ringBuffSizeEnd)
	}

	// transfer size must never be negative
	if transferSize < 0 {
		Log(LOG_ERR, "Generator %d: ring buffer transfer size < 0", gen.id)
	}

	// get pcie bar
	pcieBAR := gen.nt.pcieBAR

	// get current read pointer position
	ringBuffRdPtr := pcieBAR.Read(ADDR_BASE_NT_GEN_REPLAY[gen.id] +
		CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_ADDR_RD)

	// do a data transfer?
	var doTransfer bool

	if ringBuffRdPtr == ringBuffWrPtr {
		// ring buffer is empty -> write data
		doTransfer = true
	} else if ringBuffRdPtr < ringBuffWrPtr {
		// as long as ring buffer contains valid data, read and write pointers
		// must never become equal. If the read pointer is smaller than the
		// write pointer, we may fill up the memory until the end. This means
		// that the write pointer will wrap around and have a value of 0. Now
		// if the read pointer is currently 0 as well, this would result in an
		// error situation in which the ring buffer would be assumed to be
		// empty. Thus, prevent this case here.
		doTransfer = (ringBuffRdPtr != 0) ||
			(uint64(ringBuffWrPtr)+uint64(transferSize)) != ringBuffSize
	} else if ringBuffRdPtr > ringBuffWrPtr {
		// to make sure that the write pointer never reaches the value of the
		// read pointer (which would mean that the ring buffer is empty) only
		// transfer data if difference between both pointers is larger than
		// the transfer size
		doTransfer = (ringBuffRdPtr - ringBuffWrPtr) > transferSize
	}

	if doTransfer == false {
		// currently we cannot transfer data
		return 0
	}

	// read data from trace file
	data := gen.trace.read(traceSize-traceSizeOutStanding, transferSize)

	// take time before starting dma transfer
	transferStartTime := time.Now()

	// write data to the ring buffer
	err := gen.nt.pcieDMAWrite.Write(gen.ringBuffAddr+uint64(ringBuffWrPtr),
		data)
	if err != nil {
		Log(LOG_ERR, err.Error())
	}

	// evaluate dma transfer time
	transferDuration := time.Since(transferStartTime)

	// update the write pointer
	if (uint64(ringBuffWrPtr) + uint64(transferSize)) == ringBuffSize {
		// end of memory reached, wrap around
		ringBuffWrPtr = 0x0
	} else if (uint64(ringBuffWrPtr) + uint64(transferSize)) > ringBuffSize {
		panic("should not happen!")
	} else {
		ringBuffWrPtr += transferSize
	}

	// save write pointer and write to hardware
	gen.ringBuffWrPtr = ringBuffWrPtr
	pcieBAR.Write(ADDR_BASE_NT_GEN_REPLAY[gen.id]+
		CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_ADDR_WR, ringBuffWrPtr)

	// increment number of transfered trace bytes
	gen.nBytesTransfered += uint64(transferSize)

	// calculate dma transfer average throughput in Gbps
	transferThroughput := 8.0 * float64(transferSize) /
		transferDuration.Seconds() / 1e9

	// print out performance metrics
	Log(LOG_DEBUG, "Generator %d: %d bytes in %s (%f Gbps)",
		gen.id, transferSize, transferDuration, transferThroughput)

	// return the amount of data that has been transferred
	return transferSize
}

// start triggers the hardware to start reading data from the TX ring buffer
// in DRAM memory. The data is transferred to a FIFO in Block RAM. As long as
// the rate control module is disabled, no data is transmitted and reading from
// the TX ring buffer pauses when the FIFO becomes full. As soon as the rate
// control module is enabled, packets are transmitted on the link and the Block
// RAM FIFO is filled with data from the TX ring buffer.
func (gen *Generator) start() {
	if gen.trace == nil {
		// nothing to do here
		return
	}

	// trigger start
	gen.nt.pcieBAR.Write(ADDR_BASE_NT_GEN_REPLAY[gen.id]+
		CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_START, 0x1)
}

// isActive returns true, if the hardware core is currently reading data from
// the TX ring buffer in the FPGA board's DRAM. Once all trace data has been
// read from the ring buffer, the function returns false. Note that at this
// point some replay data may still be in the Block RAM FIFO waiting to be sent
// to the MAC by the rate control module. Thus, the rate control module should
// not be disabled immediately.
func (gen *Generator) isActive() bool {
	status := gen.nt.pcieBAR.Read(ADDR_BASE_NT_GEN_REPLAY[gen.id] +
		CPUREG_OFFSET_NT_GEN_REPLAY_STATUS)
	return (status & 0x3) > 0
}

// checkError checks if the hardware flagged an error during replay. If the
// parameter exit is set to true, the application exits if an error was
// detected.
func (gen *Generator) checkError(exit bool) error {
	// check rate control module errors
	status := gen.nt.pcieBAR.Read(ADDR_BASE_NT_GEN_RATE_CTRL[gen.id] +
		CPUREG_OFFSET_NT_GEN_RATE_CTRL_STATUS)
	if (status & 0x1) > 0 {
		if exit {
			Log(LOG_ERR, "Generator %d: replay timing error", gen.id)
		}
		return fmt.Errorf("Generator %d: replay timing error", gen.id)
	}

	// no error!
	return nil
}

// resetHardware resets the hardware core.
func (gen *Generator) resetHardware() {
	// nothing to do here.
}

// freeHostMemory resets the pointer pointing to the trace data.
func (gen *Generator) freeHostMemory() {
	gen.trace = nil
}
