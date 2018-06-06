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
// This file defines the Generators data type, a slice containing pointers
// on a list of Generator instances. It implements some convenience functions
// enabling easy control over a set of generators on more than one network
// interface.

package gofluent10g

import (
	"github.com/aoeldemann/gopcie"
)

// Generators is a slice type holding pointers on Generator instances. It
// implements functions that allow easy control of multiple Generator instances
// at once.
type Generators []*Generator

// resetHardware resets the hardware cores.
func (gens *Generators) resetHardware() {
	for _, gen := range *gens {
		gen.resetHardware()
	}
}

// configHardware initializes the configuration of the generators and writes it
// to the hardware.
func (gens *Generators) configHardware() {
	for _, gen := range *gens {
		gen.configHardware()
	}
}

// start triggers the hardware to start reading data from the TX ring buffers
// in DRAM memory. The data is transferred to FIFOs in Block RAM. As long as
// the rate control modules are disabled, no data is transmitted and reading
// from the TX ring buffers pauses when the respective FIFO becomes full. As
// soon as the rate control modules are enabled, packets are transmitted on the
// link and the Block RAM FIFOs are filled with data from the TX ring buffer.
func (gens *Generators) start() {
	for _, gen := range *gens {
		gen.start()
	}
}

// writeRingBuffs writes trace data to the TX ring buffer of the generators in
// the DRAM memory of the FPGA board. It returns the total number of bytes that
// have been for all configured generators.
func (gens *Generators) writeRingBuffs() uint64 {
	var nTransferedBytes uint64
	for _, gen := range *gens {
		nTransferedBytes += uint64(gen.writeRingBuff())
	}
	return nTransferedBytes
}

// startRateCtrl activates the rate control modules on all configured
// generators.
func (gens *Generators) startRateCtrl(pcieBAR *gopcie.PCIeBAR) {
	// assemble interface mask
	ifMask := uint32(0)

	// activate generator rate control module's for synch. start
	for _, gen := range *gens {
		var enable uint32
		if gen.trace != nil {
			enable = 0x1
		} else {
			enable = 0x0
		}
		ifMask = (ifMask & ^(0x1 << uint(gen.id))) | (enable << uint(gen.id))
	}
	pcieBAR.Write(ADDR_BASE_NT_CTRL+
		CPUREG_OFFSET_NT_CTRL_RATE_CTRL_ACTIVE, ifMask)
}

// stopRateCtrl deactivates the rate control modules on all configured
// generators.
func (gens *Generators) stopRateCtrl(pcieBAR *gopcie.PCIeBAR) {
	pcieBAR.Write(ADDR_BASE_NT_CTRL+
		CPUREG_OFFSET_NT_CTRL_RATE_CTRL_ACTIVE, 0x0)
}

// areActive returns true, if one or more hardware cores are currently reading
// data from their TX ring buffer in the FPGA board's DRAM. Once all trace data
// has been read from the ring buffers, the function returns false. Note that
// at this point some replay data may still be in the Block RAM FIFOs waiting to
// be sent to the MACs by the rate control modules. Thus, the rate control
// modules should not be disabled immediately.
func (gens *Generators) areActive() bool {
	for _, gen := range *gens {
		if gen.trace != nil && gen.isActive() {
			return true
		}
	}
	return false
}

// checkErrors checks if the hardware flagged an error during replay. If the
// parameter exit is set to true, the application exits if an error was
// detected.
func (gens *Generators) checkErrors(exit bool) error {
	for _, gen := range *gens {
		err := gen.checkError(exit)
		if err != nil {
			return err
		}
	}
	return nil
}

// getIfIdsConfigured returns a list containing the interface IDs of the
// generators that are configured to replay a trace file.
func (gens *Generators) getIfIdsConfigured() []int {
	var ids []int
	for _, gen := range *gens {
		if gen.trace != nil {
			ids = append(ids, gen.id)
		}
	}
	return ids
}
