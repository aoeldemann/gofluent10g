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
// Implements the exchange of JSON messages with the Fluent10G agent runnning
// on the device-under-test (DuT) via a ZeroMQ-based communication channel. By
// using this module, the measurement application can trigger events that cause
// actions (e.g. reconfiguration) to be executed on the DuT. In return, it can
// collect monitoring information recorded by software running on the DuT.

package dut

import (
	"encoding/json"
	"fmt"

	"github.com/aoeldemann/gofluent10g"
	zmq "github.com/pebbe/zmq4"
)

// DeviceUnderTest is a struct providing methods for interaction with the
// Device-under-Test.
type DeviceUnderTest struct {
	Name     string      // name of the DuT
	hostname string      // hostname of the DuT
	port     uint16      // port number on which the DuT agent is listening
	sock     *zmq.Socket // ZMQ socket
}

// dutMsg is the base JSON message struct for messages that are sent to the DuT.
type dutMsg struct {
	EvtName string `json:"evt_name"` // event name
}

// DeviceUnderTestCreate creates and initializes a new DeviceUnderTest struct.
func DeviceUnderTestCreate(name, hostname string, port uint16) DeviceUnderTest {
	dut := DeviceUnderTest{
		Name:     name,
		hostname: hostname,
		port:     port,
	}
	return dut
}

// Connect establishes the connection with the DuT.
func (dut *DeviceUnderTest) Connect() {
	// create zmq socket
	var sock *zmq.Socket
	sock, err := zmq.NewSocket(zmq.REQ)
	if err != nil {
		gofluent10g.Log(gofluent10g.LOG_ERR,
			"DuT '%s': could not create socket", dut.Name)
	}

	// connect to device endpoint
	err = sock.Connect(fmt.Sprintf("tcp://%s:%d", dut.hostname, dut.port))
	if err != nil {
		gofluent10g.Log(gofluent10g.LOG_ERR, "DuT '%s': could not connect",
			dut.Name)
	}

	// save socket
	dut.sock = sock

	gofluent10g.Log(gofluent10g.LOG_DEBUG,
		"DuT '%s': socket connected (tcp://%s:%d)", dut.Name, dut.hostname,
		dut.port)
}

// Disconnect closes the connection with the DuT.
func (dut *DeviceUnderTest) Disconnect() {
	// only disconnect if connection established
	if dut.sock != nil {
		// disconnect
		err := dut.sock.Disconnect(
			fmt.Sprintf("tcp://%s:%d", dut.hostname, dut.port))

		if err != nil {
			gofluent10g.Log(gofluent10g.LOG_ERR,
				"DuT '%s': could not disconnect", dut.Name)
		}

		// reset socket
		dut.sock = nil

		gofluent10g.Log(gofluent10g.LOG_DEBUG, "DuT '%s': disconnected",
			dut.Name)
	}
}

// TriggerEvent triggers a remote DuT event. The function expects the event
// name and a JSON argument struct. The parameter blocking determines whether
// the function call should block until the DuT acknowledged the event trigger.
// For blocking event calls, the function returns return data that can
// optionally be provided by the DuT. For non-blocking calls, the function
// always return nil.
func (dut *DeviceUnderTest) TriggerEvent(evtName string, args interface{},
	blocking bool) interface{} {
	gofluent10g.Log(gofluent10g.LOG_DEBUG,
		"DuT '%s': triggering '%s' event ...", dut.Name, evtName)

	// preparte json message to be sent
	type dutMsgArgs struct {
		dutMsg
		Args interface{} `json:"args"`
	}

	// create message
	msg := dutMsgArgs{}
	msg.EvtName = evtName
	msg.Args = args

	// send message
	dut.sendMsg(msg)

	// initialize return data
	var returnData interface{}

	if blocking {
		// wait for DuT response
		returnData = dut.recvRespMsg()
	} else {
		// non-blocking call, so we are not waiting for return data
		returnData = nil
	}

	gofluent10g.Log(gofluent10g.LOG_DEBUG,
		"DuT '%s': sucessfully triggered '%s' event", dut.Name, evtName)

	return returnData
}

// WaitEventCompleted waits until outstanding non-blocking event triggers
// are completed.
func (dut *DeviceUnderTest) WaitEventCompleted() {
	// wait for DuT response
	dut.recvRespMsg()
}

// GetMonitorData fetches and returns monitoring data from the DuT. The
// function expects the identifier of the data that shall be fetched.
func (dut *DeviceUnderTest) GetMonitorData(ident string) interface{} {
	// set up event arguments
	args := struct {
		Ident string `json:"ident"`
	}{
		Ident: ident,
	}

	// trigger the blocking execution of the 'get_monitor_data' event. the
	// event's return data contains the requested data.
	return dut.TriggerEvent("get_monitor_data", args, true)
}

// sendMsg transmits an event message to the DuT.
func (dut *DeviceUnderTest) sendMsg(msg interface{}) {
	// make sure connection is active
	if dut.sock == nil {
		gofluent10g.Log(gofluent10g.LOG_ERR,
			"DUT '%s': no connection active", dut.Name)
	}

	// marshal json message
	data, err := json.Marshal(msg)
	if err != nil {
		gofluent10g.Log(gofluent10g.LOG_ERR,
			"DuT '%s': failed to encode json message", dut.Name)
	}

	// send message to dut
	if _, err := dut.sock.SendBytes(data, 0); err != nil {
		gofluent10g.Log(gofluent10g.LOG_ERR,
			"DuT '%s': failed to send message to DuT", dut.Name)
	}
}

// recvRespMsg receives a response message (ACK/NACK) from the DuT. If the DuT
// answers with a NACK, the function raises an error containing the error
// message that the DuT sent.
func (dut *DeviceUnderTest) recvRespMsg() interface{} {
	// make sure connection is active
	if dut.sock == nil {
		gofluent10g.Log(gofluent10g.LOG_ERR,
			"DUT '%s': no connection active", dut.Name)
	}

	// wait for response from dut
	data, err := dut.sock.RecvBytes(0)
	if err != nil {
		gofluent10g.Log(gofluent10g.LOG_ERR,
			"DuT '%s': failed to received response message", dut.Name)
	}

	// unmarshal json message
	var respMsg dutMsg
	json.Unmarshal(data, &respMsg)

	if respMsg.EvtName == "nack" {
		// received message is a nack, so some kind of error occured on the
		// dut-side. convert message to extract the reason from the json
		// message.
		type dutMsgNack struct {
			dutMsg
			Args struct {
				Reason string `json:"reason"`
			} `json:"args"`
		}

		// unmarshal json message
		var respMsgNack dutMsgNack
		json.Unmarshal(data, &respMsgNack)

		// raise error reported by the dut
		gofluent10g.Log(gofluent10g.LOG_ERR, "DuT '%s': DuT reported: '%s'",
			dut.Name, respMsgNack.Args.Reason)

		// no return data
		return nil
	} else if respMsg.EvtName == "ack" {
		// message is a ack. In some cases, return data may be provided.
		// convert message and extract it from JSON data
		type dutMsgAck struct {
			dutMsg
			Args struct {
				ReturnData interface{} `json:"return_data"`
			} `json:"args"`
		}

		// unmarshal json message
		var respMsgAck dutMsgAck
		json.Unmarshal(data, &respMsgAck)
		return respMsgAck.Args.ReturnData
	} else {
		gofluent10g.Log(gofluent10g.LOG_ERR,
			"DuT '%s': received message with invalid event name", dut.Name)
		return nil
	}
}
