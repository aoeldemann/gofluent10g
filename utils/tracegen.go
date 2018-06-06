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
// Implements several functions for synthetic trace generation.

package utils

import (
	"encoding/binary"
	"github.com/aoeldemann/gofluent10g"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"math"
	"math/rand"
	"net"
	"runtime"
	"time"
)

// GenTraceCBR generates traffic with constant bit rate and packet lenghts.
// Only Ethernet and IPv4 headers are generated, payload bits are set to zero.
// datarate defines the target data rate, pktlenWire the length of the
// generated packets. pktlenCapture defines the number of data bytes that are
// written to the hardware, the hardware adds pktlenWire - pktlenCapture zero
// bytes to restore the original packet length. Since the Ethernet FCS is not
// included in the trace, but appended by the MAC, the condition pktlenCapture
// + 4 >= pktlenWire must always hold true. The duration parameter specifies
// the total duration of the generated trace. The parameter nRepeats determins
// how often the generated trace shall be replayed. Data is only generated for
// the first replay, the generator wraps around for all further replays.
func GenTraceCBR(datarate float64, pktlenWire, pktlenCapture int, duration time.Duration, nRepeats int) *gofluent10g.Trace {
	// MAC will append FCS, so substract 4 bytes from wire length
	pktlenWire -= 4

	// check if capture length is valid
	if pktlenCapture > pktlenWire {
		gofluent10g.Log(gofluent10g.LOG_ERR, "GenTraceCBR: invalid capture length")
	}

	// calculate the number of packets we will generate
	// (+8 byte for preamble + SOD, +12 byte for inter-frame gap, +4 byte for
	// FCS) => add 24 bytes
	nPkts := round(duration.Seconds() * datarate / float64(8*(pktlenWire+24)))

	gofluent10g.Log(gofluent10g.LOG_DEBUG, "Generating %d packets", nPkts)

	// we will reuse the same ethernet header for all packets, generate source
	// and destination MAC addresses
	macSrc, _ := net.ParseMAC("53:00:00:00:00:01")
	macDst, _ := net.ParseMAC("53:00:00:00:00:02")

	// generate ethernet header
	hdrEth := &layers.Ethernet{
		SrcMAC:       macSrc,
		DstMAC:       macDst,
		EthernetType: layers.EthernetTypeIPv4,
	}

	// determine the mean inter-packet time in clock cycles. Again, packet
	// length does not include Ethernet preamble, SOD and FCS, as well as
	// inter-frame gap => add 24 bytes
	cyclesInterPacketMean := gofluent10g.FREQ_SFP * float64(8*(pktlenWire+24)) /
		datarate

	// generate random source and destination ipv4 addresses, which will be
	// reused for all packets
	ipSrc := make([]byte, 4)
	ipDst := make([]byte, 4)
	rand.Read(ipSrc)
	rand.Read(ipDst)

	// create ip header (substract 14 bytes for ethernet header, i.e. mac
	// addresses and ethertype) from ip packet length
	hdrIp := &layers.IPv4{
		Version: 4,
		IHL:     5,
		Length:  uint16(pktlenWire - 14),
		SrcIP:   ipSrc,
		DstIP:   ipDst,
	}

	// serialize packet data
	bufPkt := gopacket.NewSerializeBuffer()
	err := gopacket.SerializeLayers(bufPkt, gopacket.SerializeOptions{},
		hdrEth, hdrIp)
	if err != nil {
		gofluent10g.Log(gofluent10g.LOG_ERR, "%s", err.Error())
	}

	// accumulated inter-packet clock cycle rounding error
	accCyclesInterPacketRoundErr := 0.0

	// accumulated number of clock cycles between packets
	accCyclesInterPacket := uint64(0)

	// create data structures for packet and meta data
	data := make([][]byte, nPkts)
	lensWire := make([]int, nPkts)
	lensCapture := make([]int, nPkts)
	cyclesInterPacket := make([]int, nPkts)

	for i := 0; i < nPkts; i++ {
		// store packet data
		data[i] = bufPkt.Bytes()

		// all packets have the same length
		lensWire[i] = pktlenWire
		lensCapture[i] = pktlenCapture

		// the average number of clock cycles between two packets is a
		// floating-point number, but clock cycles must always be integer
		// values. If we always round up we are sending too slow, if we always
		// round down we send too fast. Sending too fast at full line-rate
		// causes timing errors. We start by rounding up and accumulate the
		// resulting rounding error. If the error becomes larger than 1 full
		// clock cycle, we round down and decrease the accumulated error for the
		// next packet. On average we will hit the target mean inter-packet
		// cycle value.
		if accCyclesInterPacketRoundErr < 1.0 {
			// not enough rounding error accumulated yet -> round up
			cyclesInterPacket[i] = int(math.Ceil(cyclesInterPacketMean))
			accCyclesInterPacketRoundErr +=
				math.Ceil(cyclesInterPacketMean) - cyclesInterPacketMean
		} else {
			// enough rounding error accumulated -> round down
			cyclesInterPacket[i] = int(math.Floor(cyclesInterPacketMean))
			accCyclesInterPacketRoundErr -=
				cyclesInterPacketMean - math.Floor(cyclesInterPacketMean)
		}

		// accumulate clock cycles between packets
		accCyclesInterPacket += uint64(cyclesInterPacket[i])
	}

	// calculate actual replay duration after rounding and print it
	actualDuration :=
		time.Duration(float64(accCyclesInterPacket)/gofluent10g.FREQ_SFP*1e9) *
			time.Nanosecond
	gofluent10g.Log(gofluent10g.LOG_DEBUG, "Actual trace duration: %s (Target was %s)",
		actualDuration, duration)

	// manually call garbage collector
	runtime.GC()

	// create trace buffer
	bufTrace := bufTraceAssemble(data, lensWire, lensCapture, cyclesInterPacket)

	// manually call garbage collector again
	runtime.GC()

	// create and return trace struct
	return gofluent10g.TraceCreateFromData(bufTrace, nPkts, actualDuration, nRepeats)
}

// GenTraceRandom generates random traffic with a mean data rate specified by
// the datarateMean (bits per second) parameter. The size of each packet is
// uniformly distributed between 64 and 1518 bytes (size of the Ethernet frame,
// i.e. starting with destination MAC address, ending with FCS). To reach the
// target mean data rate, a gap is inserted between packets. The length of the
// packet is determined by an exponential distribution. pktlenCaptureMax
// defines the maximum number of data bytes that are written to the hardware.
// Hardware then appends zero bytes to the packet to restore its original
// length. The duration parameter specifies the total duration of the generated
// trace. The parameter nRepeats determins how often the generated trace shall
// be replayed. Data is only generated for the first replay, the generator
// wraps around for all further replays.
func GenTraceRandom(datarateMean float64, pktlenCaptureMax int, duration time.Duration, nRepeats int) *gofluent10g.Trace {
	// packet length is uniformly distributed between 64 and 1518 bytes. Since
	// MAC will append FCS, the packets we generate here are 4 bytes shorter
	pktlenMin := 60
	pktlenMax := 1514
	pktlenMean := (pktlenMin + pktlenMax) / 2

	// calculate the average time of the gap between two packets (add 24 bytes
	// for FCS, preamble, SOD and inter-frame gap
	tGapMean := float64(8*(pktlenMean+24))/datarateMean -
		float64(8*(pktlenMean+24))/10e9

	// calculate the number of packets we will generate. add 24 bytes to the
	// packet length to account for Ethernet preamble + SOD, inter-frame gap
	// and FCS
	nPkts := round(duration.Seconds() * datarateMean /
		float64(8*(pktlenMean+24)))

	gofluent10g.Log(gofluent10g.LOG_DEBUG, "Generating %d packets", nPkts)

	// we will reuse the same ethernet header for all packets
	macSrc, _ := net.ParseMAC("53:00:00:00:00:01")
	macDst, _ := net.ParseMAC("53:00:00:00:00:02")

	// create ethernet header
	hdrEth := &layers.Ethernet{
		SrcMAC:       macSrc,
		DstMAC:       macDst,
		EthernetType: layers.EthernetTypeIPv4,
	}

	// generate random source and destination ipv4 address, which will be
	// reused for all packets
	ipSrc := make([]byte, 4)
	ipDst := make([]byte, 4)
	rand.Read(ipSrc)
	rand.Read(ipDst)

	// create ip header
	hdrIp := &layers.IPv4{
		Version: 4,
		IHL:     5,
		SrcIP:   ipSrc,
		DstIP:   ipDst,
	}

	// accumulated inter-packet clock cycle rounding error
	accCyclesInterPacketRoundErr := 0.0

	// accumulated number of clock cycles between packets
	accCyclesInterPacket := uint64(0)

	// create data structures for packet and meta data
	data := make([][]byte, nPkts)
	lensWire := make([]int, nPkts)
	lensCapture := make([]int, nPkts)
	cyclesInterPacket := make([]int, nPkts)

	for i := 0; i < nPkts; i++ {
		// determine packet length according to uniform distribution
		lensWire[i] = rand.Intn(pktlenMax-pktlenMin+1) + pktlenMin

		// set capture length
		if lensWire[i] < pktlenCaptureMax {
			lensCapture[i] = lensWire[i]
		} else {
			lensCapture[i] = pktlenCaptureMax
		}

		// calculate the number of cycles it takes to transmit the packet
		cyclesTransfer := gofluent10g.FREQ_SFP * float64(8*(lensWire[i]+24)) / 10e9

		// random gap between packets
		cyclesGap := gofluent10g.FREQ_SFP * tGapMean * rand.ExpFloat64()

		// add both cycle numbers up
		cyclesTotal := cyclesTransfer + cyclesGap

		// hardware does not support inter-packet cycle numbers larger than
		// 32 bit, so cut if necessary
		if cyclesTotal > 4294967295 {
			cyclesTotal = 4294967295
		}

		// the average number of clock cycles between two packets is a
		// floating-point number, but clock cycles must always be integer
		// values. If we always round up we are sending too slow, if we always
		// round down we send too fast. Sending too fast at full line-rate
		// causes timing errors. We start by rounding up and accumulate the
		// resulting rounding error. If the error becomes larger than 1 full
		// clock cycle, we round down and decrease the accumulated error for the
		// next packet. On average we will hit the target mean inter-packet
		// cycle value.
		if accCyclesInterPacketRoundErr < 1.0 {
			// not enough rounding error accumulated yet -> round up
			cyclesInterPacket[i] = int(math.Ceil(cyclesTotal))
			accCyclesInterPacketRoundErr +=
				math.Ceil(cyclesTotal) - cyclesTotal
		} else {
			// enough rounding error accumulated -> round down
			cyclesInterPacket[i] = int(math.Floor(cyclesTotal))
			accCyclesInterPacketRoundErr -=
				cyclesTotal - math.Floor(cyclesTotal)
		}

		// accumulate clock cycles between packets
		accCyclesInterPacket += uint64(cyclesInterPacket[i])

		// set IPv4 packet length in header (substract 14 bytes for ethernet
		// header, i.e. mac addresses and ethertype)
		hdrIp.Length = uint16(lensWire[i] - 14)

		// serialize packet data
		bufPkt := gopacket.NewSerializeBuffer()
		err := gopacket.SerializeLayers(bufPkt, gopacket.SerializeOptions{},
			hdrEth, hdrIp)
		if err != nil {
			gofluent10g.Log(gofluent10g.LOG_ERR, "%s", err.Error())
		}

		// add packet to list
		data[i] = bufPkt.Bytes()
	}

	// calculate actual replay duration after rounding and print it
	actualDuration :=
		time.Duration(float64(accCyclesInterPacket)/gofluent10g.FREQ_SFP*1e9) *
			time.Nanosecond
	gofluent10g.Log(gofluent10g.LOG_DEBUG, "Actual trace duration: %s (Target was %s)",
		actualDuration, duration)

	// manually call garbage collector
	runtime.GC()

	// create trace buffer
	bufTrace := bufTraceAssemble(data, lensWire, lensCapture, cyclesInterPacket)

	// manually call garbage collector again
	runtime.GC()

	// create and return trace
	return gofluent10g.TraceCreateFromData(bufTrace, nPkts, actualDuration, nRepeats)
}

// bufTraceAssemble creates the content of a trace file that can be replayed
// by a generator of the network tester. It expects the packet data. Also, for
// each packet the packet wire and capture length, as well as the inter-packet
// time in clock cycles must be specified.
func bufTraceAssemble(data [][]byte, lensWire, lensCapture, cyclesInterPacket []int) []byte {
	// calculate total amount of trace data we need to write to the hardware
	bufTraceSize := 8 * int64(len(data)) // 8 byte meta information per packet

	for i := 0; i < len(data); i++ {
		// capture data aligned to 8 byte
		if lensCapture[i]%8 == 0 {
			bufTraceSize += int64(lensCapture[i])
		} else {
			bufTraceSize += int64(8 * (lensCapture[i]/8 + 1))
		}
	}

	// allign to 32 byte
	if bufTraceSize%64 != 0 {
		bufTraceSize = 64 * (bufTraceSize/64 + 1)
	}

	// allocate trace buffer memory
	bufTrace := make([]byte, bufTraceSize)

	// initialize buffer write address
	bufTraceAddr := int64(0)

	for i := 0; i < len(data); i++ {
		// assemble meta data
		meta := uint64(cyclesInterPacket[i])
		meta |= uint64(lensCapture[i]) << 32
		meta |= uint64(lensWire[i]) << 48

		// write meta data word
		binary.LittleEndian.PutUint64(bufTrace[bufTraceAddr:bufTraceAddr+8],
			meta)
		bufTraceAddr += 8

		// write packet data
		if len(data[i]) > lensCapture[i] {
			copy(bufTrace[bufTraceAddr:bufTraceAddr+int64(lensCapture[i])],
				data[i][0:lensCapture[i]])
		} else {
			copy(bufTrace[bufTraceAddr:bufTraceAddr+int64(len(data[i]))],
				data[i])
		}

		if lensCapture[i]%8 == 0 {
			bufTraceAddr += int64(lensCapture[i])
		} else {
			bufTraceAddr += int64(8 * (lensCapture[i]/8 + 1))
		}
	}

	// add padding for 32 byte alignment
	for bufTraceAddr%64 != 0 {
		binary.LittleEndian.PutUint64(bufTrace[bufTraceAddr:bufTraceAddr+8],
			0xFFFFFFFFFFFFFFFF)
		bufTraceAddr += 8
	}

	return bufTrace
}

// round rounds a floating-point number to the next integer value
func round(x float64) int {
	return int(math.Floor(x + 0.5))
}

// ceils rounds a floating-point number to the next higher integer value
func ceil(x float64) int {
	return int(math.Ceil(x))
}
