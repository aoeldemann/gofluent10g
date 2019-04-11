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

import (
	"time"
)

// Interface is the struct providing methods for obtaining the number of packets
// that have been transmitted and received by the network interface.
type Interface struct {
	nt                     *NetworkTester
	id                     int
	datarateSampleInterval time.Duration
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

// SetDatarateSampleInterval sets the sample interval with which the hardware
// should evaluate the RX and TX data rates on the interface.
func (iface *Interface) SetDatarateSampleInterval(sampleInterval time.Duration) {
	// store sample interval
	iface.datarateSampleInterval = sampleInterval

	// convert sample interval to number of clock cycles
	sampleIntervalCycles := uint32(sampleInterval.Seconds() * FREQ_SFP)

	// write sample interval to hardware
	iface.nt.pcieBAR.Write(ADDR_BASE_NT_DATARATE[iface.id]+
		CPUREG_OFFSET_NT_DATARATE_CTRL_SAMPLE_INTERVAL, sampleIntervalCycles)
}

// GetDatrateTX returns the nominal and raw TX data rates observed at the
// interface in the last second (in Gbps).
func (iface *Interface) GetDatrateTX() (float64, float64) {
	// get number of bytes transmitted in last sample interval
	nBytes := iface.nt.pcieBAR.Read(ADDR_BASE_NT_DATARATE[iface.id] +
		CPUREG_OFFSET_NT_DATARATE_STATUS_TX_N_BYTES)
	nBytesRaw := iface.nt.pcieBAR.Read(ADDR_BASE_NT_DATARATE[iface.id] +
		CPUREG_OFFSET_NT_DATARATE_STATUS_TX_N_BYTES_RAW)

	// return nominal and raw datarates
	return 8.0 * float64(nBytes) / iface.datarateSampleInterval.Seconds() / 1e9,
		8.0 * float64(nBytesRaw) / iface.datarateSampleInterval.Seconds() / 1e9
}

// GetDatrateRX returns the nominal and raw RX data rates observed at the
// interface in the last second (in Gbps).
func (iface *Interface) GetDatrateRX() (float64, float64) {
	nBytes := iface.nt.pcieBAR.Read(ADDR_BASE_NT_DATARATE[iface.id] +
		CPUREG_OFFSET_NT_DATARATE_STATUS_RX_N_BYTES)
	nBytesRaw := iface.nt.pcieBAR.Read(ADDR_BASE_NT_DATARATE[iface.id] +
		CPUREG_OFFSET_NT_DATARATE_STATUS_RX_N_BYTES_RAW)

	// return nominal and raw datarates
	return 8.0 * float64(nBytes) / iface.datarateSampleInterval.Seconds() / 1e9,
		8.0 * float64(nBytesRaw) / iface.datarateSampleInterval.Seconds() / 1e9
}

// resetHardware resets the network interfaces.
func (iface *Interface) resetHardware() {
	// nothing to do here
}
