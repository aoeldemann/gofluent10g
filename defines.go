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
// Global definitions.

package gofluent10g

const (
	// SFP+ clock domain frequency
	FREQ_SFP = 156.25e6

	// PCIExpress Base Address Register IDs
	PCIE_BAR_FUNCTION_ID = 0x0
	PCIE_BAR_VENDOR_ID   = 0x10ee
	PCIE_BAR_DEVICE_ID   = 0x7032
	PCIE_BAR_ID          = 0x0

	// number of network interfaces
	N_INTERFACES = 4

	// number of clock cycles that shall pass between two timestamp counter
	// increments (default value)
	TIMESTAMP_CNTR_CYCLES_PER_TICK_DEFAULT = 1

	// expected hardware identification values. will be checked upon
	// initialization
	HW_CRC16   = 0xf15e
	HW_VERSION = 0x000d

	// maximum size of a ring buffer write
	RING_BUFF_WR_TRANSFER_SIZE_MAX = 64 * 1024 * 1024

	// minimum size of a ring buffer read
	RING_BUFF_RD_TRANSFER_SIZE_MIN = 64 * 1024 * 1024

	// amount of host memory that is reserved for capture data for each network
	// interface on which capturing is enabled (default value)
	CAPTURE_HOST_MEM_SIZE_DEFAULT = 4 * 1024 * 1024 * 1024
)

// PCIExpress device names
const (
	PCIE_XDMA_DEV_H2C = "/dev/xdma0_h2c_0"
	PCIE_XDMA_DEV_C2H = "/dev/xdma0_c2h_0"
)

// DRAM memory addresses and ranges
const (
	ADDR_DDR_A       = uint64(0x000000000)
	ADDR_DDR_B       = uint64(0x100000000)
	ADDR_RANGE_DDR_A = uint32(0xFFFFFFFF)
	ADDR_RANGE_DDR_B = uint32(0xFFFFFFFF)
)

// peripheral base addresses
var (
	ADDR_BASE_NT_GEN_REPLAY = []uint32{
		0x00000000,
		0x00001000,
		0x00002000,
		0x00003000,
	}

	ADDR_BASE_NT_GEN_RATE_CTRL = []uint32{
		0x00004000,
		0x00005000,
		0x00006000,
		0x00007000,
	}

	ADDR_BASE_NT_CTRL = uint32(0x00008000)

	ADDR_BASE_NT_RECV_CAPTURE = []uint32{
		0x00009000,
		0x0000A000,
		0x0000B000,
		0x0000C000,
	}

	ADDR_BASE_NT_RECV_FILTER_MAC = []uint32{
		0x0000D000,
		0x0000E000,
		0x0000F000,
		0x00010000,
	}

	ADDR_BASE_IFACE = []uint32{
		0x00011000,
		0x00012000,
		0x00013000,
		0x00014000,
	}

	ADDR_BASE_NT_TIMESTAMP = uint32(0x00015000)
	ADDR_BASE_NT_IDENT     = uint32(0x00016000)
)

// peripheral register offsets
const (
	CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_MEM_ADDR_LO   = uint32(0x00000000)
	CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_MEM_ADDR_HI   = uint32(0x00000004)
	CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_MEM_RANGE     = uint32(0x00000008)
	CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_TRACE_SIZE_LO = uint32(0x0000000C)
	CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_TRACE_SIZE_HI = uint32(0x00000010)
	CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_ADDR_WR       = uint32(0x00000014)
	CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_ADDR_RD       = uint32(0x00000018)
	CPUREG_OFFSET_NT_GEN_REPLAY_CTRL_START         = uint32(0x0000001C)
	CPUREG_OFFSET_NT_GEN_REPLAY_STATUS             = uint32(0x00000020)

	CPUREG_OFFSET_NT_GEN_RATE_CTRL_STATUS = uint32(0x00000000)

	CPUREG_OFFSET_NT_CTRL_RATE_CTRL_ACTIVE = uint32(0x00000000)
	CPUREG_OFFSET_NT_CTRL_RST              = uint32(0x00000004)

	CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_ACTIVE          = uint32(0x00000000)
	CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_MEM_ADDR_LO     = uint32(0x00000004)
	CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_MEM_ADDR_HI     = uint32(0x00000008)
	CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_MEM_RANGE       = uint32(0x0000000C)
	CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_ADDR_WR         = uint32(0x00000010)
	CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_ADDR_RD         = uint32(0x00000014)
	CPUREG_OFFSET_NT_RECV_CAPTURE_CTRL_MAX_LEN_CAPTURE = uint32(0x00000018)
	CPUREG_OFFSET_NT_RECV_CAPTURE_STATUS_PKT_CNT       = uint32(0x0000001C)
	CPUREG_OFFSET_NT_RECV_CAPTURE_STATUS_ACTIVE        = uint32(0x00000020)
	CPUREG_OFFSET_NT_RECV_CAPTURE_STATUS_ERRS          = uint32(0x00000024)

	CPUREG_OFFSET_NT_RECV_FILTER_MAC_CTRL_ADDR_DST_HI      = uint32(0x00000000)
	CPUREG_OFFSET_NT_RECV_FILTER_MAC_CTRL_ADDR_DST_LO      = uint32(0x00000004)
	CPUREG_OFFSET_NT_RECV_FILTER_MAC_CTRL_ADDR_MASK_DST_HI = uint32(0x00000008)
	CPUREG_OFFSET_NT_RECV_FILTER_MAC_CTRL_ADDR_MASK_DST_LO = uint32(0x0000000C)

	CPUREG_OFFSET_IF_N_PKTS_TX = uint32(0x00000000)
	CPUREG_OFFSET_IF_N_PKTS_RX = uint32(0x00000004)

	CPUREG_OFFSET_NT_TIMESTAMP_CYCLES_PER_TICK = uint32(0x00000000)
	CPUREG_OFFSET_NT_TIMESTAMP_MODE            = uint32(0x00000004)
	CPUREG_OFFSET_NT_TIMESTAMP_POS             = uint32(0x00000008)
	CPUREG_OFFSET_NT_TIMESTAMP_WIDTH           = uint32(0x0000000C)

	CPUREG_OFFSET_NT_IDENT_IDENT = uint32(0x00000000)
)
