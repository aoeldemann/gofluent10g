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
// This file implements the Interface struct. It provides functions to obtain
// the number of packets that have been transmitted and received by the
// respective network interface.

package gofluent10g

// Interface is the struct providing methods for obtaining the number of packets
// that have been transmitted and received by the network interface.
type Interface struct {
	nt *NetworkTester
	id int
}

// GetPacketCountRX returns the number of packets received on the interface.
func (iface *Interface) GetPacketCountRX() int {
	nPkts := iface.nt.pcieBAR.Read(
		ADDR_BASE_IFACE[iface.id] + CPUREG_OFFSET_IF_N_PKTS_RX)
	return int(nPkts)
}

// GetPacketCountTX returns the number of packets transmitted on the interface.
func (iface *Interface) GetPacketCountTX() int {
	nPkts := iface.nt.pcieBAR.Read(
		ADDR_BASE_IFACE[iface.id] + CPUREG_OFFSET_IF_N_PKTS_TX)
	return int(nPkts)
}

// resetHardware resets the network interfaces.
func (iface *Interface) resetHardware() {
	// nothing to do here
}
