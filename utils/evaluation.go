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
// Implements several functions for statistical evaluation.

package utils

import (
	"github.com/aoeldemann/gofluent10g"
	"math"
	"sort"
)

// LatencyHistogram defines a struct that contains information on how often
// each latency value was recorded.
type LatencyHistogram []struct {
	Latency     float64
	Occurrences int
}

// LatencyCDF describes the data structure containing CDF data.
type LatencyCDF []struct {
	Latency, Probability float64
}

// CalcLatencyMean calculates the mean latency based on a slice of captured
// packets.
func CalcLatencyMean(pkts gofluent10g.CapturePackets) float64 {
	// get packet latencies
	latencies := pkts.GetLatencies()

	// return -1.0 if the slice is empty
	if len(latencies) == 0 {
		return -1.0
	}

	var latencyTotal float64
	for _, latency := range latencies {
		latencyTotal += latency
	}

	return latencyTotal / float64(len(latencies))
}

// CalcLatencyStdDev calculates the latency standard deviation based on a slice
// of captured packets.
func CalcLatencyStdDev(pkts gofluent10g.CapturePackets, latencyMean float64) float64 {
	// get latencies
	latencies := pkts.GetLatencies()

	// return -1.0 if the slice is empty
	if len(latencies) == 0 {
		return -1.0
	}

	var latencyStdDev float64
	for _, latency := range latencies {
		latencyStdDev += math.Pow(latency-latencyMean, 2)
	}
	latencyStdDev = math.Sqrt(latencyStdDev / float64(len(latencies)))
	return latencyStdDev
}

// CalcLatencyHistogram calculates the latency histogram based on a slice
// of captured packets. It returns the latency histogram as well as the total
// number of latency values.
func CalcLatencyHistogram(pkts gofluent10g.CapturePackets) (LatencyHistogram, int) {
	// get list of latencies
	latencies := pkts.GetLatencies()

	// create map to record per-latency occurrences
	latencyMap := map[float64]int{}

	for _, latency := range latencies {
		if _, ok := latencyMap[latency]; ok {
			latencyMap[latency] += 1
		} else {
			latencyMap[latency] = 1
		}
	}

	// get unique latency values
	uniqueLatencies := []float64{}
	for latency, _ := range latencyMap {
		uniqueLatencies = append(uniqueLatencies, latency)
	}

	// sort unique latencies in ascending order
	sort.Sort(sort.Float64Slice(uniqueLatencies))

	// create histogram struct
	latencyHistogram := make(LatencyHistogram, len(uniqueLatencies))

	// append histogram entires
	for i, latency := range uniqueLatencies {
		latencyHistogram[i].Latency = latency
		latencyHistogram[i].Occurrences = latencyMap[latency]
	}

	return latencyHistogram, len(latencies)
}

// CalcLatencyCDF calculates a latency CDF based on a slice of captured packets.
func CalcLatencyCDF(pkts gofluent10g.CapturePackets) LatencyCDF {
	// create latency histogram
	latencyHistogram, nLatencies := CalcLatencyHistogram(pkts)

	// create latency cdf structure
	latencyCDF := make(LatencyCDF, len(latencyHistogram))

	occurrencesAccumulated := 0

	for i, histElement := range latencyHistogram {
		occurrencesAccumulated += histElement.Occurrences

		latencyCDF[i].Latency = histElement.Latency
		latencyCDF[i].Probability = float64(occurrencesAccumulated) /
			float64(nLatencies)
	}

	return latencyCDF
}
