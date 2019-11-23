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
// This file defines the Receivers data type, a slice containing pointers
// on a list of Receiver instances. It implements some convenience functions
// enabling easy control over a set of receivers on more than one network
// interface.

package gofluent10g

import "github.com/aoeldemann/gopcie"

// Receivers is a slice type holding pointers on Receiver instacnes. It
// implements functions that allow easy control of multiple Receiver instances
// at once.
type Receivers []*Receiver

// resetHardware resets the hardware cores.
func (recvs *Receivers) resetHardware() {
	for _, recv := range *recvs {
		recv.resetHardware()
	}
}

// configHardware initializes the configuration of the receivers and writes it
// to the hardware.
func (recvs *Receivers) configHardware() {
	for _, recv := range *recvs {
		recv.configHardware()
	}
}

// start starts the continous reading of data from the ring buffers. The
// function is non-blocking.
func (recvs *Receivers) start() {
	for _, recv := range *recvs {
		recv.start()
	}
}

// stop stops the reading of data from the ring buffers.
func (recvs *Receivers) stop() {
	for _, recv := range *recvs {
		recv.stop()
	}
}

// readRingBuff reads data from the ring buffer. The PCI Express DMA device
// through which the read shall be performed needs to be provided as an
// argument.
func (recvs *Receivers) readRingBuffs(pcieDMA *gopcie.PCIeDMA) {
	for _, recv := range *recvs {
		recv.readRingBuff(false, pcieDMA)
	}
}

// checkError checks if one or more hardware receiver core flagged an error
// during operation. The function returns and error if one was detected. If the
// parameter exit is set to true, the function prints an error message and
// exits the application if an error was detected.
func (recvs *Receivers) checkErrors(exit bool) error {
	for _, recv := range *recvs {
		err := recv.checkError(exit)
		if err != nil {
			return err
		}
	}
	return nil
}

// getIfIdsConfigured returns a list containing the interface IDs of the
// receivers that configured to capture the arriving packets.
func (recvs *Receivers) getIfIdsConfigured() []int {
	var ids []int
	for _, recv := range *recvs {
		if recv.captureEnable {
			ids = append(ids, recv.id)
		}
	}
	return ids
}
