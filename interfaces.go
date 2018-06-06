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
// This file defines the Interfaces data type, a slice containing pointers
// on a list of Interface instances. It implements some convenience functions
// enabling easy access to the packet counter values of more than one network
// interface at once.

package gofluent10g

// Interfaces is a slice type holding pointers on Interface instances. It
// implements functions that allow easy access to packet counter values of
// multiple Interface instances at once.
type Interfaces []*Interface

// GetPacketCountRX returns the total number of received packets.
func (ifaces *Interfaces) GetPacketCountRX() int {
	nPkts := 0
	for _, iface := range *ifaces {
		nPkts += iface.GetPacketCountRX()
	}
	return nPkts
}

// GetPacketCountTX returns the total number of transmitted packets.
func (ifaces *Interfaces) GetPacketCountTX() int {
	nPkts := 0
	for _, iface := range *ifaces {
		nPkts += iface.GetPacketCountTX()
	}
	return nPkts
}

// resetHardware resets the network interfaces.
func (ifaces *Interfaces) resetHardware() {
	for _, iface := range *ifaces {
		iface.resetHardware()
	}
}
