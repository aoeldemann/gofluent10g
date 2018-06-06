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
// Implements several functions that operate on a list of captured packets.

package gofluent10g

// CapturePackets is a slice containing CapturePacket structs.
type CapturePackets []CapturePacket

// GetLatencies returns a list containing the recorded latency for timestamped
// packets.
func (pkts CapturePackets) GetLatencies() []float64 {
	var latencies []float64

	for _, pkt := range pkts {
		if pkt.HasLatency {
			latencies = append(latencies, pkt.Latency)
		}
	}

	return latencies
}

// GetArrivalTimes returns a list containing the recorded packet arrival times.
func (pkts CapturePackets) GetArrivalTimes() []float64 {
	arrivalTimes := make([]float64, len(pkts))

	for i := 0; i < len(pkts); i++ {
		arrivalTimes[i] = pkts[i].ArrivalTime
	}

	return arrivalTimes
}

// CapturePacketsSortByLatency implements sorting of CapturePackets based
// on the recorded latency.
type CapturePacketsSortByLatency CapturePackets

func (pkts CapturePacketsSortByLatency) Len() int {
	return len(pkts)
}

func (pkts CapturePacketsSortByLatency) Swap(i, j int) {
	pkts[i], pkts[j] = pkts[j], pkts[i]
}

func (pkts CapturePacketsSortByLatency) Less(i, j int) bool {
	return pkts[i].Latency < pkts[j].Latency
}
