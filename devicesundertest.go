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
// Implements communication with multipe devices-under-test at once.

package gofluent10g

// DevicesUnderTest is a slice type holding DeviceUnderTest structs. The.
// receiver functions defined on it allow easy control of multiple DuTs at once.
type DevicesUnderTest []DeviceUnderTest

// Disconnect closes the connection with the DuTs.
func (duts *DevicesUnderTest) Disconnect() {
	for _, dut := range *duts {
		dut.Disconnect()
	}
}

// TriggerEvent triggers a remote DuT event on all DuTs. The function expects
// the event type name and a JSON argument struct. The parameter blocking
// determines whether the function call should block until the DuTs acknowledged
// the event triggers.
func (duts *DevicesUnderTest) TriggerEvent(evtType string, args interface{},
	blocking bool) {
	for _, dut := range *duts {
		dut.TriggerEvent(evtType, args, blocking)
	}
}

// WaitAllEventsCompleted waits for all outstanding non-blocking event triggers
// on all DuTs to complete.
func (duts *DevicesUnderTest) WaitAllEventsCompleted() {
	for _, dut := range *duts {
		dut.WaitEventCompleted()
	}
}
