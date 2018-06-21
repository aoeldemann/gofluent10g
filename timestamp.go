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
// File implements functionality to configure the latency timestamp hardware
// core. The core provides a timestamp value for insertion and extraction
// to/from packet data for packet latency calculation. By default, the counter
// is incremented every clock cyle. It can optionally be configured to
// increment at a slower rate. Additionally, the core can be configured to
// either insert the timestamp in the IPv4 or IPv6 header (checksum and
// flowlabel fields) or at a fixed byte position.

package gofluent10g

// timestamp is the struct representing the latency timestamp counter
// hardware core.
type timestamp struct {
	nt *NetworkTester

	cyclesPerTick int
	mode          int
	pos           int
	width         int
}

const (
	TimestampModeDisabled int = 0
	TimestampModeFixedPos int = 1
	TimestampModeHeader   int = 2
)

// setCyclesPerTick sets the number of clock cycles that shall pass between
// two counter increments.
func (timestamp *timestamp) setCyclesPerTick(cycles int) {
	timestamp.cyclesPerTick = cycles
}

// getTickPeriod returns the time period (in seconds) that passes between
// two counter increments.
func (timestamp *timestamp) getTickPeriod() float64 {
	return float64(timestamp.cyclesPerTick) / FREQ_SFP
}

// setMode selects the timestamp insertion/extraction mode. If the mode is set
// to 'TimestampModeHeader', the timestamp is inserted into the header of IPv4
// or IPv6 packets (checksum or flowtlabel field respectively). If the mode is
// set to 'TimestampModeFixedPos', the timestamp is inserted in the packet data
// at a configurable byte position. If the mode is set to
// 'TimestampModeDisabled', no timestamp is inserted at all.
func (timestamp *timestamp) setMode(mode int) {
	if mode != TimestampModeFixedPos && mode != TimestampModeHeader {
		Log(LOG_ERR, "Timestamp: invalid timestamping mode")
	}
	timestamp.mode = mode
}

// setPos specifies the byte position where the timestamp shall be inserted in
// the packet data. It requires the timestamping mode to be set to
// 'TimestampModeFixedPos'.
func (timestamp *timestamp) setPos(pos int) {
	if timestamp.mode != TimestampModeFixedPos {
		Log(LOG_ERR, "Timestamp: cannot set timestamp position when mode is "+
			"not set to 'TimestampModeFixedPos'")
	}
	if pos < 0 || pos > 1518 {
		Log(LOG_ERR, "Timestamp: invalid timestamp position")
	}
	timestamp.pos = pos
}

// setWidth sets the width of the timestamp that is inserted in the packet.
// Currently the values 16 and 24 (bits) are supported.
func (timestamp *timestamp) setWidth(width int) {
	if timestamp.mode != TimestampModeFixedPos {
		Log(LOG_ERR, "Timestamp: cannot set timestamp when when mode is "+
			"not set to 'TimestampModeFixedPos'")
	}
	if width != 16 && width != 24 {
		Log(LOG_ERR, "Timestamp: timestamp width must be either 16 or 24 bit")
	}
	timestamp.width = width
}

// configHardware writes the configuration to the hardware.
func (timestamp *timestamp) configHardware() {
	if timestamp.mode == TimestampModeFixedPos {
		if timestamp.width == 16 {
			// timestamp position valid? currently timestamps may not spread
			// across two 8 byte data words
			if timestamp.pos%8 > 6 {
				Log(LOG_ERR, "Timestamp: invalid timestamp position")
			}

			// write timestamp width
			timestamp.nt.pcieBAR.Write(ADDR_BASE_NT_TIMESTAMP+
				CPUREG_OFFSET_NT_TIMESTAMP_WIDTH, 0x0)
		} else if timestamp.width == 24 {
			// timestamp position valid? currently timestamps may not spread
			// across two 8 byte data words
			if timestamp.pos%8 > 5 {
				Log(LOG_ERR, "Timestamp: invalid timestamp position")
			}

			// write timestamp width
			timestamp.nt.pcieBAR.Write(ADDR_BASE_NT_TIMESTAMP+
				CPUREG_OFFSET_NT_TIMESTAMP_WIDTH, 0x1)
		} else {
			Log(LOG_ERR, "Timestamp: timestamp width not configured")
		}

		// write timestamp position
		timestamp.nt.pcieBAR.Write(ADDR_BASE_NT_TIMESTAMP+
			CPUREG_OFFSET_NT_TIMESTAMP_POS, uint32(timestamp.pos))
	} else if timestamp.mode == TimestampModeHeader {
		// reset timestamp position and width to zero
		timestamp.nt.pcieBAR.Write(ADDR_BASE_NT_TIMESTAMP+
			CPUREG_OFFSET_NT_TIMESTAMP_POS, 0x0)
		timestamp.nt.pcieBAR.Write(ADDR_BASE_NT_TIMESTAMP+
			CPUREG_OFFSET_NT_TIMESTAMP_WIDTH, 0x0)
	} else if timestamp.mode == TimestampModeDisabled {
		// reset timestamp position and width to zero
		timestamp.nt.pcieBAR.Write(ADDR_BASE_NT_TIMESTAMP+
			CPUREG_OFFSET_NT_TIMESTAMP_POS, 0x0)
		timestamp.nt.pcieBAR.Write(ADDR_BASE_NT_TIMESTAMP+
			CPUREG_OFFSET_NT_TIMESTAMP_WIDTH, 0x0)
	} else {
		Log(LOG_ERR, "Timestamp: invalid mode")
	}

	// write timestamp mode
	timestamp.nt.pcieBAR.Write(ADDR_BASE_NT_TIMESTAMP+
		CPUREG_OFFSET_NT_TIMESTAMP_MODE, uint32(timestamp.mode))

	// set timestamp tick interval
	timestamp.nt.pcieBAR.Write(ADDR_BASE_NT_TIMESTAMP+
		CPUREG_OFFSET_NT_TIMESTAMP_CYCLES_PER_TICK,
		uint32(timestamp.cyclesPerTick))

	// print some debug messages
	Log(LOG_DEBUG, "Timestamp: %d clock cycles per tick (%.2f ns)",
		timestamp.cyclesPerTick, timestamp.getTickPeriod()*1e9)
	if timestamp.mode == TimestampModeFixedPos {
		Log(LOG_DEBUG, "Timestamp: mode 'TimestampModeFixedPos'")
		Log(LOG_DEBUG, "Timestamp: pos: %d, width: %d", timestamp.pos,
			timestamp.width)
	} else if timestamp.mode == TimestampModeHeader {
		Log(LOG_DEBUG, "Timestamp: mode 'TimestampModeHeader'")
	} else if timestamp.mode == TimestampModeDisabled {
		Log(LOG_DEBUG, "Timestamp: mode 'TimestampModeDisabled'")
	}
}
