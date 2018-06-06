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
// Implements communication with the device-under-test via a zeromq-based
// connection. By using this module, the network tester can trigger certain
// events that cause actions to be executed by a software running on the
// Device-under-Test. This allows the network tester to trigger a configuration
// update on the DuT and thus to evalute the impacts of the DuT configuration on
// the measured performance.

package gofluent10g

import (
	"encoding/json"
	"fmt"
	zmq "github.com/pebbe/zmq4"
	"net"
)

// DeviceUnderTest is a struct providing methods for interaction with the
// Device-under-Test.
type DeviceUnderTest struct {
	Name string      // name of the DuT
	ip   net.IP      // IP address on which the DuT agent is listening
	port uint16      // Port number on which the DuT agent is listening
	sock *zmq.Socket // ZMQ socket
}

// dutMsg is a JSON message that is sent to the DuT.
type dutMsg struct {
	EvtType string `json:"evtType"`
}

// DeviceUnderTestCreate creates and initializes new DeviceUnderTest struct.
func DeviceUnderTestCreate(name string, ip net.IP, port uint16) DeviceUnderTest {
	dut := DeviceUnderTest{
		Name: name,
		ip:   ip,
		port: port,
	}

	return dut
}

// Connect establishes the connection with the DuT.
func (dut *DeviceUnderTest) Connect() {
	// create zmq socket
	var sock *zmq.Socket
	sock, err := zmq.NewSocket(zmq.REQ)
	if err != nil {
		Log(LOG_ERR, "DuT '%s': could not create socket", dut.Name)
	}

	// connect to device endpoint
	err = sock.Connect(fmt.Sprintf("tcp://%s:%d", dut.ip.String(), dut.port))
	if err != nil {
		Log(LOG_ERR, "DuT '%s': could not connect", dut.Name)
	}

	// save socket
	dut.sock = sock

	Log(LOG_DEBUG, "DuT '%s': connected (tcp://%s:%d)",
		dut.Name, dut.ip, dut.port)
}

// Disconnect closes the connection with the DuT.
func (dut *DeviceUnderTest) Disconnect() {
	// only disconnect if connection established
	if dut.sock != nil {
		// disconnect
		err := dut.sock.Disconnect(
			fmt.Sprintf("tcp://%s:%d", dut.ip.String(), dut.port))

		if err != nil {
			Log(LOG_ERR, "DuT '%s': could not disconnect", dut.Name)
		}

		Log(LOG_DEBUG, "DuT '%s': disconnected", dut.Name)
	}
}

// TriggerEvent triggers a remote DuT event. The function expects the event
// type name and a JSON argument struct. The parameter blocking determines
// whether the function call should block until the DuT acknowledged the event
// trigger. For blocking event calls, the function returns return data that
// can optionally be provided by the DuT. For non-blocking calls, the function
// always return nil.
func (dut *DeviceUnderTest) TriggerEvent(evtType string, args interface{},
	blocking bool) interface{} {
	// preparte json message to be sent
	type dutMsgArgs struct {
		dutMsg
		Args interface{} `json:"args"`
	}

	// create message
	msg := dutMsgArgs{}
	msg.EvtType = evtType
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

	Log(LOG_DEBUG, "DuT '%s': triggered %s event", dut.Name, evtType)

	return returnData
}

// WaitEventCompleted waits until acknowledgements for all event triggers that
// have been issued non-blocking are received.
func (dut *DeviceUnderTest) WaitEventCompleted() {
	// wait for DuT response
	dut.recvRespMsg()
}

// sendMsg transmits an event message to the DuT.
func (dut *DeviceUnderTest) sendMsg(msg interface{}) {
	// marshal json message
	data, err := json.Marshal(msg)
	if err != nil {
		Log(LOG_ERR, "DuT '%s': failed to encode json message", dut.Name)
	}

	// send message to dut
	if _, err := dut.sock.SendBytes(data, 0); err != nil {
		Log(LOG_ERR, "DuT '%s': failed to send message to DuT", dut.Name)
	}
}

// recvRespMsg receives a response message (ACK/NACK) from the DuT. If the DuT
// answers with a NACK, the function raises an error containing the error
// message that the DuT sent.
func (dut *DeviceUnderTest) recvRespMsg() interface{} {
	// wait for response from dut
	data, err := dut.sock.RecvBytes(0)
	if err != nil {
		Log(LOG_ERR,
			"DuT '%s': failed to received response message", dut.Name)
	}

	// unmarshal json message
	var respMsg dutMsg
	json.Unmarshal(data, &respMsg)

	if respMsg.EvtType == "nack" {
		// received message is a NACK, so some kind of error occur on the
		// DuT-side. Convert message to extract the reason from the JSON
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
		Log(LOG_ERR, "DuT '%s': DuT reported: '%s'", dut.Name,
			respMsgNack.Args.Reason)

		return nil
	} else if respMsg.EvtType == "ack" {
		// message is a ACK. In some cases, return data may be provided.
		// convert message and extract it from JSON data
		type dutMsgAck struct {
			dutMsg
			Args struct {
				ReturnData interface{} `json:"returnData"`
			} `json:"args"`
		}

		// unmarshal json message
		var respMsgAck dutMsgAck
		json.Unmarshal(data, &respMsgAck)
		return respMsgAck.Args.ReturnData
	} else {
		Log(LOG_ERR, "DuT '%s': received message with invalid message type",
			dut.Name)
		return nil
	}
}
