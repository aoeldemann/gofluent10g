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
// This file implements the Receiver struct, which configures and controls the
// capture hardware. The hardware is structed as follows:
//
//  -----       ----------       ------       --------------
// | MAC | --> | MAC Addr | --> | BRAM | --> | DRAM         |
// |     |     | Filter   |     | FIFO |     | RX Ring Buff |
//  -----       ----------       ------       --------------

package gofluent10g

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

// Receiver is the struct providing methods for configuring the traffic capture
// hardware cores. Each instance of the struct corresponds to one hardware
// core on one network interface. The struct additionally provides methods for
// transferring data from the RX ring buffers in the DRAM of the FPGA board to
// the host memory.
type Receiver struct {
	nt *NetworkTester
	id int

	captureEnable  bool // determines whether capturing is enabled
	captureDiscard bool // determines whehter capture data will be discarded
	captureMaxLen  int  // maximum packet data capture length

	capture            *Capture // capture instance
	captureHostMemSize uint64   // host memory size for capture data

	// ring buffer memory address, size and read pointer position
	ringBuffAddr      uint64
	ringBuffAddrRange uint32 // ring buffer must never be larger than 4 GByte
	ringBuffRdPtr     uint32

	// packet filter destination MAC address and mask
	filterMACAddrDst     net.HardwareAddr
	filterMACAddrMaskDst uint64
}

// EnableCapture enables/disables capturing of packet and/or meta data.
func (recv *Receiver) EnableCapture(enable bool) {
	recv.captureEnable = enable
}

// SetCaptureDiscard enalbes/disables the discarding of captured data. If
// the function is enabled, the capture data will be fetched from the hardware,
// but will be discarded after it has been fetched. This is helpful for
// debugging and network tester performance analysis without consuming huge
// amounts of memory for capture data.
func (recv *Receiver) SetCaptureDiscard(discard bool) {
	if recv.captureEnable == false {
		Log(LOG_ERR, "Receiver %d: could not enable/disable capture discard, "+
			"because capturing is disabled", recv.id)
	}
	recv.captureDiscard = discard
}

// SetCaptureMaxLen sets the maximum number of per-packet capture bytes.
func (recv *Receiver) SetCaptureMaxLen(maxLen int) {
	if recv.captureEnable == false {
		Log(LOG_ERR, "Receiver %d: could not set maximum capture length, "+
			"because capturing is disabled", recv.id)
	}

	recv.captureMaxLen = maxLen
}

// SetCaptureHostMemSize sets the size of the host memory that shall be reserved
// for capture data.
func (recv *Receiver) SetCaptureHostMemSize(size uint64) {
	if recv.captureEnable == false {
		Log(LOG_ERR, "Receiver %d: could not set capture host memory size, "+
			"because capturing is disabled", recv.id)
	}

	// 64 byte alignment
	if size%64 == 0 {
		recv.captureHostMemSize = size
	} else {
		recv.captureHostMemSize = 64 * (size/64 + 1)
	}
}

// GetCapture returns Capture instance assigned to the receiver.
func (recv *Receiver) GetCapture() *Capture {
	if recv.captureEnable == false {
		Log(LOG_ERR, "Receiver %d: could not get Capture struct, because "+
			"capturing is disabled", recv.id)
	}

	return recv.capture
}

// SetFilterMacAddrDst sets the destination MAC address and mask by which
// arriving packets shall be filtered.
func (recv *Receiver) SetFilterMacAddrDst(addr string, addrMask uint64) {
	if recv.captureEnable == false {
		Log(LOG_ERR, "Receiver %d: could not set filter destination MAC "+
			"address, because capturing is disabled", recv.id)
	}

	filterMACAddrDst, err := net.ParseMAC(addr)
	if err != nil {
		Log(LOG_ERR,
			"Receiver %d: could not parse filter mac destination address",
			recv.id)
	}

	if addrMask > 0xFFFFFFFFFFFF {
		Log(LOG_ERR, "Receiver %d: invalid mac address filter destination mask",
			recv.id)
	}

	recv.filterMACAddrDst = filterMACAddrDst
	recv.filterMACAddrMaskDst = addrMask
}

// DisableFilterMacAddrDst disables packet filtering by MAC destination address.
func (recv *Receiver) DisableFilterMacAddrDst() {
	if recv.captureEnable == false {
		Log(LOG_ERR, "Receiver %d: could not clear filter destination MAC "+
			"address, because capturing is disabled", recv.id)
	}

	recv.filterMACAddrDst = nil
}

// GetPacketCountCaptured returns the number of packets that were captured.
func (recv *Receiver) GetPacketCountCaptured() int {
	if recv.captureEnable == false {
		Log(LOG_ERR, "Receiver %d: could not obtain number of captured "+
			"packets, because capturing is disabled", recv.id)
	}

	nPkts := recv.nt.pcieBAR.Read(ADDR_BASE_NT_RECV_CAPTURE[recv.id] +
		CPUREG_OFFSET_NT_RECV_CAPTURE_STATUS_PKT_CNT)
	return int(nPkts)
}

// configHardware writes the software configuration of the receiver down to the
// hardware core.
func (recv *Receiver) configHardware() {
	// nothing to do here if no capturing is activated for this receiver
	if recv.captureEnable == false {
		return
	}

	// calculate ring buffer size
	ringBuffSize := uint64(recv.ringBuffAddrRange) + 1

	// the ring buffer size must be larger than 16384 bytes
	if ringBuffSize <= 16384 {
		Log(LOG_ERR,
			"Receiver %d: ring buffer size must be larger than 16384 bytes.",
			recv.id)
	}

	// the ring buffer size must be a multiple of (2*8192) bytes
	if ringBuffSize%16384 != 0 {
		Log(LOG_ERR,
			"Receiver %d: ring buffer size must be a multiple of 16384 bytes.",
			recv.id)
	}

	// the ring buffer transfer size must be a multiple of 16384 bytes
	if RING_BUFF_RD_TRANSFER_SIZE_MIN%16384 != 0 {
		Log(LOG_ERR,
			"Receiver %d: ring buffer transfer size must be a multiple of "+
				"16384 bytes.", recv.id)
	}

	// the ring buffer transfer size must be smaller than the ring buffer
	// size
	if ringBuffSize <= RING_BUFF_RD_TRANSFER_SIZE_MIN {
		Log(LOG_ERR,
			"Receiver %d: ring buffer transfer size must be smaller than ring "+
				"buffer size", recv.id)
	}

	// get pcie bar access module
	pcieBAR := recv.nt.pcieBAR

	// write ring buffer memory region infos to receiver
	pcieBAR.Write(ADDR_BASE_NT_RECV_CAPTURE[recv.id]+
		CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_MEM_ADDR_HI,
		uint32(recv.ringBuffAddr>>32))
	pcieBAR.Write(ADDR_BASE_NT_RECV_CAPTURE[recv.id]+
		CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_MEM_ADDR_LO,
		uint32(recv.ringBuffAddr&0xFFFFFFFF))
	pcieBAR.Write(ADDR_BASE_NT_RECV_CAPTURE[recv.id]+
		CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_MEM_RANGE,
		recv.ringBuffAddrRange)

	// reset ring buffer read pointer
	recv.ringBuffRdPtr = 0x0
	pcieBAR.Write(ADDR_BASE_NT_RECV_CAPTURE[recv.id]+
		CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_ADDR_RD, recv.ringBuffRdPtr)

	Log(LOG_DEBUG,
		"Receiver %d: capturing to ring buffer addr 0x%016x, range 0x%016x",
		recv.id, recv.ringBuffAddr, recv.ringBuffAddrRange)

	// configure maximum capture length
	pcieBAR.Write(ADDR_BASE_NT_RECV_CAPTURE[recv.id]+
		CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_MAX_LEN_CAPTURE,
		uint32(recv.captureMaxLen))

	Log(LOG_DEBUG,
		"Receiver %d: capturing up to %d bytes of packet data", recv.id,
		recv.captureMaxLen)

	// create capture memory instance
	recv.capture = captureCreate(recv.nt, recv.captureHostMemSize,
		recv.nt.timestamp.getTickPeriod(), recv.captureMaxLen,
		recv.captureDiscard)

	// setup mac address filter dst
	if recv.filterMACAddrDst != nil {
		addrMaskByte := make([]byte, 8)
		binary.BigEndian.PutUint64(addrMaskByte, recv.filterMACAddrMaskDst)

		addrHi := binary.LittleEndian.Uint16(recv.filterMACAddrDst[4:6])
		addrLo := binary.LittleEndian.Uint32(recv.filterMACAddrDst[0:4])

		addrMaskHi := binary.LittleEndian.Uint16(addrMaskByte[6:8])
		addrMaskLo := binary.LittleEndian.Uint32(addrMaskByte[2:6])

		recv.nt.pcieBAR.Write(ADDR_BASE_NT_RECV_FILTER_MAC[recv.id]+
			CPUREG_OFFSET_NT_RECV_FILTER_MAC_CTRL_ADDR_DST_HI, uint32(addrHi))

		recv.nt.pcieBAR.Write(ADDR_BASE_NT_RECV_FILTER_MAC[recv.id]+
			CPUREG_OFFSET_NT_RECV_FILTER_MAC_CTRL_ADDR_DST_LO, addrLo)

		recv.nt.pcieBAR.Write(ADDR_BASE_NT_RECV_FILTER_MAC[recv.id]+
			CPUREG_OFFSET_NT_RECV_FILTER_MAC_CTRL_ADDR_MASK_DST_HI,
			uint32(addrMaskHi))

		recv.nt.pcieBAR.Write(ADDR_BASE_NT_RECV_FILTER_MAC[recv.id]+
			CPUREG_OFFSET_NT_RECV_FILTER_MAC_CTRL_ADDR_MASK_DST_LO,
			addrMaskLo)
	} else {
		recv.nt.pcieBAR.Write(ADDR_BASE_NT_RECV_FILTER_MAC[recv.id]+
			CPUREG_OFFSET_NT_RECV_FILTER_MAC_CTRL_ADDR_MASK_DST_HI, 0)

		recv.nt.pcieBAR.Write(ADDR_BASE_NT_RECV_FILTER_MAC[recv.id]+
			CPUREG_OFFSET_NT_RECV_FILTER_MAC_CTRL_ADDR_MASK_DST_LO, 0)
	}
}

// readRingBuff reads capture data from the receiver's RX ring buffer in the
// DRAM of the FPGA board. It returns the number of bytes that have been
// transferred. Transfers only occur if at least RING_BUFF_RD_TRANSFER_SIZE_MIN
// bytes are present in the ring buffer or if the number of bytes to be read
// until the end of the ring buffer are smaller than
// RING_BUFF_RD_TRANSFER_SIZE_MIN. If the parameter readAll is set to true, the
// minimum transfer size is ignored and the function reads as many bytes as it
// can get.
func (recv *Receiver) readRingBuff(readAll bool) uint32 {
	if recv.captureEnable == false {
		// nothing to do here
		return 0
	}

	// get the ring buffer size
	ringBuffSize := uint64(recv.ringBuffAddrRange) + 1

	// get the current read pointer value
	ringBuffRdPtr := recv.ringBuffRdPtr

	// calculate the memory size of the ring buffer from the current position
	// of the read pointer until the end
	ringBuffSizeEnd := ringBuffSize - uint64(ringBuffRdPtr)

	// get pcie bar
	pcieBAR := recv.nt.pcieBAR

	// get current write pointer position
	ringBuffWrPtr := pcieBAR.Read(ADDR_BASE_NT_RECV_CAPTURE[recv.id] +
		CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_ADDR_WR)

	// calculate target transfer size
	var transferSize uint32
	if ringBuffSizeEnd <= RING_BUFF_RD_TRANSFER_SIZE_MIN {
		transferSize = uint32(ringBuffSizeEnd)
	} else {
		transferSize = RING_BUFF_RD_TRANSFER_SIZE_MIN
	}

	if readAll {
		// readAll parameter has been set, so we read all available data (until
		// end of ring buffer)
		if ringBuffRdPtr < ringBuffWrPtr {
			transferSize = ringBuffWrPtr - ringBuffRdPtr
		} else if ringBuffRdPtr > ringBuffWrPtr {
			transferSize = uint32(ringBuffSizeEnd)
		}
	}

	// transfer size must never be negative
	if transferSize < 0 {
		Log(LOG_ERR, "Receiver %d: ring buffer transfer size < 0", recv.id)
	}

	// do a transfer?
	var doTransfer bool

	if ringBuffRdPtr == ringBuffWrPtr {
		// ring buffer is empty -> nothing to transfer
		doTransfer = false
	} else if ringBuffRdPtr < ringBuffWrPtr {
		// we can read if the difference between both pointers is at least
		// the desired transfer size
		doTransfer = (ringBuffWrPtr - ringBuffRdPtr) >= transferSize
	} else if ringBuffRdPtr > ringBuffWrPtr {
		// we can read until the end of the ring buffer
		doTransfer = true
	}

	if doTransfer == false {
		// currently we cannot transfer data
		return 0
	}

	// get slice to which capture data shall be recorded to
	data := recv.capture.getWriteSlice(transferSize)

	// take time before starting dma transfer
	transferStartTime := time.Now()

	// read data from the ring buffer
	recv.nt.pcieDMARead.Read(recv.ringBuffAddr+uint64(ringBuffRdPtr), data)

	// evaluate dma transfer time
	transferDuration := time.Since(transferStartTime)

	// update the read pointer
	if (uint64(ringBuffRdPtr) + uint64(transferSize)) == ringBuffSize {
		// end of memory reached, wrap around
		ringBuffRdPtr = 0x0
	} else if (uint64(ringBuffRdPtr) + uint64(transferSize)) > ringBuffSize {
		panic("should not happen")
	} else {
		ringBuffRdPtr += transferSize
	}

	// save read pointer and write to hardware
	recv.ringBuffRdPtr = ringBuffRdPtr
	pcieBAR.Write(ADDR_BASE_NT_RECV_CAPTURE[recv.id]+
		CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_ADDR_RD,
		ringBuffRdPtr)

	// calculate dma transfer average throughput in Gbps
	transferThroughput := 8.0 * float64(transferSize) /
		transferDuration.Seconds() / 1e9

	// print out performance metrics
	Log(LOG_DEBUG, "Receiver %d: %d bytes in %s (%f Gbps)",
		recv.id, transferSize, transferDuration, transferThroughput)

	// return the amount of data that has been transferred
	return transferSize
}

// start starts the continous reading of data from the ring buffer. The
// function is non-blocking.
func (recv *Receiver) start() {
	if recv.captureEnable == false {
		// nothing to do here
		return
	}

	// start capturing
	recv.nt.pcieBAR.Write(ADDR_BASE_NT_RECV_CAPTURE[recv.id]+
		CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_ACTIVE, 0x1)
}

// stop stops the reading of data from the ring buffer.
func (recv *Receiver) stop() {
	if recv.captureEnable == false {
		// nothing to do here
		return
	}

	// stop capturing
	recv.nt.pcieBAR.Write(ADDR_BASE_NT_RECV_CAPTURE[recv.id]+
		CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_ACTIVE, 0x0)

	// wait a little bit to give receiver time to become inactive and flush
	// its fifo contents to the memory
	time.Sleep(time.Second)

	// make sure module is inactive
	active := recv.nt.pcieBAR.Read(ADDR_BASE_NT_RECV_CAPTURE[recv.id] +
		CPUREG_OFFSET_NT_RECV_CAPTURE_STATUS_ACTIVE)
	if active != 0x0 {
		Log(LOG_ERR, "Receiver %d: module does not become inactive", recv.id)
	}
}

// checkError checks if the hardware flagged an error during capturing. If the
// parameter exit is set to true, the application exits if an error was
// detected.
func (recv *Receiver) checkError(exit bool) error {
	errs := recv.nt.pcieBAR.Read(ADDR_BASE_NT_RECV_CAPTURE[recv.id] +
		CPUREG_OFFSET_NT_RECV_CAPTURE_STATUS_ERRS)
	if (errs & 0x1) > 0 {
		if exit {
			Log(LOG_ERR, "Receiver %d: meta FIFO full", recv.id)
		}
		return fmt.Errorf("Receiver %d: meta FIFO full", recv.id)
	}
	if (errs & 0x2) > 0 {
		if exit {
			Log(LOG_ERR, "Receiver %d: data FIFO full", recv.id)
		}
		return fmt.Errorf("Receiver %d: data FIFO full", recv.id)
	}
	return nil
}

// resetHardware resets the hardware core
func (recv *Receiver) resetHardware() {
	// disable capturing (just in case it's still active from a previous
	// errornous measurement)
	recv.nt.pcieBAR.Write(ADDR_BASE_NT_RECV_CAPTURE[recv.id]+
		CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_ACTIVE, 0x0)
}

// freeHostMemory resets the pointer pointing to capture data.
func (recv *Receiver) freeHostMemory() {
	recv.capture = nil
}
