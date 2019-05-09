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
// Toplevel network tester struct, which has methods for issuing resets,
// configuring the hardware, performing version checks, assigning ring buffer
// memory regions, ... It additionally provides methods to access generator,
// receiver and interface instances. A new network tester struct is created
// by calling the NetworkTesterCreate() function.

package gofluent10g

import (
	"runtime"
	"sync"
	"time"

	"github.com/aoeldemann/gopcie"
)

// NetworkTester is the toplevel struct providing methods for configuring the
// network tester. It provides methods that provide access to generator,
// receiver, interface and timestamp counter submodules.
type NetworkTester struct {
	pcieBAR      *gopcie.PCIeBAR
	pcieDMAWrite *gopcie.PCIeDMA
	pcieDMARead  *gopcie.PCIeDMA

	gens      Generators // slice of *Generator
	recvs     Receivers  // slice of *Receiver
	ifaces    Interfaces // slice of *Interface
	timestamp *timestamp

	syncCapture sync.WaitGroup
	stopCapture chan bool

	syncPrintDatarate sync.WaitGroup
	stopPrintDatarate chan bool

	checkErrors bool
}

// NetworkTesterCreate create a new instance of the NetworkTester struct.
func NetworkTesterCreate() *NetworkTester {
	// open PCIExpress BAR
	pcieBAR, err := gopcie.PCIeBAROpen(
		PCIE_BAR_FUNCTION_ID,
		PCIE_BAR_VENDOR_ID,
		PCIE_BAR_DEVICE_ID,
		PCIE_BAR_ID)
	if err != nil {
		Log(LOG_ERR, err.Error())
	}

	// open PCIExpress DMA for writing
	pcieDMAWrite, err := gopcie.PCIeDMAOpen(PCIE_XDMA_DEV_H2C,
		gopcie.PCIE_ACCESS_WRITE)
	if err != nil {
		Log(LOG_ERR, err.Error())
	}

	// open PCIExpress DMA for reading
	pcieDMARead, err := gopcie.PCIeDMAOpen(PCIE_XDMA_DEV_C2H,
		gopcie.PCIE_ACCESS_READ)
	if err != nil {
		Log(LOG_ERR, err.Error())
	}

	// create instance of NetworkTester struct
	nt := NetworkTester{
		pcieBAR:      pcieBAR,
		pcieDMAWrite: pcieDMAWrite,
		pcieDMARead:  pcieDMARead,
		// always enable error checking, can be disabled by the user later
		checkErrors: true,
	}

	// make sure hardware version matches software version
	nt.checkVersion()

	// create generator, receiver, interface and control instances. one per
	// network interface
	nt.gens = make(Generators, N_INTERFACES)
	nt.recvs = make(Receivers, N_INTERFACES)
	nt.ifaces = make(Interfaces, N_INTERFACES)

	for i := 0; i < N_INTERFACES; i++ {
		nt.gens[i] = &Generator{
			nt: &nt,
			id: i,
		}
		nt.recvs[i] = &Receiver{
			nt:                 &nt,
			id:                 i,
			captureHostMemSize: CAPTURE_HOST_MEM_SIZE_DEFAULT,
		}
		nt.ifaces[i] = &Interface{
			nt: &nt,
			id: i,
		}
	}

	// create timestamp core instance
	nt.timestamp = &timestamp{
		nt:            &nt,
		cyclesPerTick: TIMESTAMP_CNTR_CYCLES_PER_TICK_DEFAULT,
		mode:          TimestampModeDisabled,
	}

	// return the created instance
	return &nt
}

// Close closes the connection to the network tester hardware.
func (nt *NetworkTester) Close() {
	nt.pcieBAR.Close()
	nt.pcieDMAWrite.Close()
	nt.pcieDMARead.Close()
}

// GetGenerator returns a generator instance by its interface ID.
func (nt *NetworkTester) GetGenerator(id int) *Generator {
	if id < 0 || id >= N_INTERFACES {
		Log(LOG_ERR, "Invalid Generator ID: %d", id)
	}

	return nt.gens[id]
}

// GetGenerators returns a slice containing all generator instances.
func (nt *NetworkTester) GetGenerators() Generators {
	return nt.gens
}

// GetReceiver returns a receiver instance by its interface ID.
func (nt *NetworkTester) GetReceiver(id int) *Receiver {
	if id < 0 || id >= N_INTERFACES {
		Log(LOG_ERR, "Invalid Receiver ID: %d", id)
	}

	return nt.recvs[id]
}

// GetReceivers returns a slice containing all receiver instances.
func (nt *NetworkTester) GetReceivers() Receivers {
	return nt.recvs
}

// GetInterface returns an interface instance by its interface ID.
func (nt *NetworkTester) GetInterface(id int) *Interface {
	if id < 0 || id >= N_INTERFACES {
		Log(LOG_ERR, "Invalid Interface ID: %d", id)
	}

	return nt.ifaces[id]
}

// GetInterfaces returns a slice containing all interface instances.
func (nt *NetworkTester) GetInterfaces() Interfaces {
	return nt.ifaces
}

// FreeHostMemory resets pointers pointing to trace and capture data and then
// manually triggers garbage collection.
func (nt *NetworkTester) FreeHostMemory() {
	for _, gen := range nt.gens {
		gen.freeHostMemory()
	}

	for _, recv := range nt.recvs {
		recv.freeHostMemory()
	}

	// manually call garbage collector
	runtime.GC()
}

// WriteConfig writes the network tester configuration down to the hardware.
// Function must be called before starting replay/capture, if configuration
// was changed.
func (nt *NetworkTester) WriteConfig() {
	// reset hardware
	nt.resetHardware()

	// print out information on which generators and receivers are configured
	// for replay/capture
	genIfIds := nt.gens.getIfIdsConfigured()
	recvIfIds := nt.recvs.getIfIdsConfigured()
	Log(LOG_DEBUG, "Replaying traffic on interfaces: %d", genIfIds)
	Log(LOG_DEBUG, "Capturing traffic on interfaces: %d", recvIfIds)

	// assign memory regions
	nt.assignMemory()

	// configure all cores
	nt.configHardware()
}

// StartReplay triggers the start of packet generation on all configured
// generators. The function blocks until generation has finished.
func (nt *NetworkTester) StartReplay() {
	Log(LOG_DEBUG, "Replay: filling up TX ring buffers ...")

	// pre-fill ring buffers
	for {
		// write data to ring buffers. function returns the total number of
		// bytes that have been transferred
		nTransferedBytes := nt.gens.writeRingBuffs()

		if nTransferedBytes == 0 {
			// no data has been transferred -> ring buffers are full or there
			// is no more data to be transferred
			break
		}
	}

	Log(LOG_DEBUG, "Replay: TX ring buffers are filled up. Starting now ...")

	// trigger generators to start reading from ring buffers
	nt.gens.start()

	// wait a little bit so transmission fifos can fill up
	time.Sleep(100 * time.Millisecond)

	// start rate control module to drain fifos and transmit packets with
	// the timing denoted in the trace
	nt.gens.startRateCtrl(nt.pcieBAR)

	for {
		// continuously fill up ring buffers
		nt.gens.writeRingBuffs()

		if nt.gens.areActive() == false {
			// all generators finished draining data from the TX ring buffers
			break
		}
	}

	//  -----------        -----------        --------------        -----
	// | DRAM TX   |      | Block RAM |      | Rate Control |      | MAC |
	// | Ring Buff | ---> | FIFO      | ---> |              | ---> |     |
	//  -----------        -----------        --------------        -----
	//
	// at this point, all trace data has been read from the ring buffers in
	// DRAM. however, it may still take some time until the rate control
	// module actually finished the transmission of all packets, since it
	// must enforce the inter-packet transmission times specified in the trace.
	// in the meantime, packet data remains buffered in the block ram fifo. we
	// wait a little bit to ensure that all packets have been sent to the MAC
	// and the block ram fifo is empty.
	//
	// TODO: waiting only a single second may be too little, if inter-packet
	// transmission times are larger than a second. choose sleep duration
	// more more dynamically in the future.
	time.Sleep(time.Second)

	// stop the rate control module. at this point no packets will be read
	// from the block ram fifo anymore
	nt.gens.stopRateCtrl(nt.pcieBAR)

	// if enabled, check the hardware's error registers. the error registers
	// are set if the rate control was not able to enforce the inter-packet
	// transmission times specified in the trace. This happens if the TX ring
	// buffer can not be refilled or read in time, or if the trace specifies
	// inter-packet transmission times that would exceed the 10 Gbps line rate
	// of the network interfaces.
	if nt.checkErrors {
		nt.gens.checkErrors(true)
	}

	Log(LOG_DEBUG, "Replay: done")
}

// StartCapture stats packet capturing on all configured interfaces. The
// function is non-blocking.
func (nt *NetworkTester) StartCapture() {
	// initialize a channel we will later use to request the stop of the
	// goroutine
	nt.stopCapture = make(chan bool)

	// start goroutine and increment waiting group for sync
	nt.syncCapture.Add(1)
	go nt.capture()
}

// StopCapture stops the capturing of packet data and packet latency on all
// configured receivers.
func (nt *NetworkTester) StopCapture() {
	// trigger the goroutine reading the ring buffers to stop and wait for it to
	// complete
	nt.stopCapture <- true
	nt.syncCapture.Wait()

	// if enabled, check the hardware's error registers. the error registers
	// are set if the RX ring buffer became full and the arriving traffic thus
	// could not be captured.
	if nt.checkErrors {
		nt.recvs.checkErrors(true)
	}
}

// SetCheckErrors enables/disables hardware error checking. By default it is
// enabled and the application aborts when an error occurs. If disabled, the
// user can utilize the CheckErrors() function to manually check for errors and
// gracefully handle them.
func (nt *NetworkTester) SetCheckErrors(checkErrors bool) {
	nt.checkErrors = checkErrors
}

// SetTimestampTickPeriod sets the period (in 6.4 ns clock cycles), which shall
// pass between two subsequent latency timestamp counter increments. Large
// values allow the measurment of large network latencies, small values increase
// the measurement accuracy.
func (nt *NetworkTester) SetTimestampTickPeriod(cyclesPerTick int) {
	// update timestamp counter configuration
	nt.timestamp.setCyclesPerTick(cyclesPerTick)
}

// SetTimestampMode selects the timestamp insertion/extraction mode. If the
// mode is set to 'TimestampModeHeader', the timestamp is inserted into the
// header of IPv4 or IPv6 packets (checksum or flowtlabel field respectively).
// If the mode is set to 'TimestampModeFixedPos', the timestamp is inserted
// in the packet data at a configurable byte position.
func (nt *NetworkTester) SetTimestampMode(mode int) {
	nt.timestamp.setMode(mode)
}

// SetTimestampPos specifies the byte position where the timestamp shall be
// inserted in the packet data. It requires the timestamping mode to be set to
// 'TimestampModeFixedPos'.
func (nt *NetworkTester) SetTimestampPos(pos int) {
	nt.timestamp.setPos(pos)
}

// SetTimestampWidth sets the width of the timestamp that is inserted in the
// packet. Currently the values 16 and 24 (bits) are supported.
func (nt *NetworkTester) SetTimestampWidth(width int) {
	nt.timestamp.setWidth(width)
}

// CheckErrors checks if the hardware flagged an error and returns an error
// if one was detected. The user is responsible to handle the error, the
// application does not abort. The function returns nil if no error occured.
func (nt *NetworkTester) CheckErrors() error {
	if err := nt.gens.checkErrors(false); err != nil {
		return err
	}
	if err := nt.recvs.checkErrors(false); err != nil {
		return err
	}

	// no error
	return nil
}

// PrintDataratesStart starts a thread that periodically prints out RX and TX
// data rates of all network interfaces. Expects data rate sampling period/
// print out frequency as parameter.
func (nt *NetworkTester) PrintDataratesStart(sampleInterval time.Duration) {
	// configure sample interval in hardware
	for _, iface := range nt.ifaces {
		iface.SetDatarateSampleInterval(sampleInterval)
	}

	// set up and start thread
	nt.stopPrintDatarate = make(chan bool)
	nt.syncPrintDatarate.Add(1)
	go nt.printDatarates(sampleInterval)
}

// PrintDataratesStop stops the thread that periodically prints out RX and TX
// data rates.
func (nt *NetworkTester) PrintDataratesStop() {
	// stop thread and wait for completion
	nt.stopPrintDatarate <- true
	nt.syncPrintDatarate.Wait()
}

// capture continuously reads the receiver ring buffers. It must be started in
// a goroutine.
func (nt *NetworkTester) capture() {
	defer nt.syncCapture.Done()

	// trigger hardware to start capturing
	nt.recvs.start()

	var stop bool
	for {
		select {
		case _ = <-nt.stopCapture:
			// goroutine stop requested
			stop = true
		default:
		}

		if stop {
			break
		}

		// read ring buffers
		nt.recvs.readRingBuffs()
	}

	// stop of the goroutine was requested -> stop capturing
	nt.recvs.stop()

	// drain remaining RX ring buffer contents
	for _, recv := range nt.recvs {
		for {
			nBytesRead := recv.readRingBuff(true)

			// no data has been read -> we are done
			if nBytesRead == 0 {
				break
			}
		}
	}
}

// assignMemory assigns the FPGA board's DDR memory regions in which the
// generation and capture ring buffers will be placed. Currently this is all
// hard-coded and needs some improvements to make it more dynamic in the future.
func (nt *NetworkTester) assignMemory() {
	// TODO: this function is currently tailored for the NetFPGA-SUME with
	// 8 GByte of memory (2x 4 GByte). For other memory configuration,
	// adjuments need to be done here (and possible to hardware as well).
	if ADDR_DDR_A != 0x0 || ADDR_DDR_B != 0x100000000 ||
		ADDR_RANGE_DDR_A != 0xFFFFFFFF || ADDR_RANGE_DDR_B != 0xFFFFFFFF {
		Log(LOG_ERR, "Current implementation only supports 2x 4 GByte "+
			"NetFPGA-SUME configuration")
	}

	// get the ids of the generators that are configured for traffic generation
	genIds := nt.gens.getIfIdsConfigured()
	nGens := len(genIds)

	// get the ids of the receivers that are configured for traffic capture
	recvIds := nt.recvs.getIfIdsConfigured()
	nRecvs := len(recvIds)

	if nGens == 0 && nRecvs == 0 {
		// nothing to do!
		return
	} else if nRecvs == 0 {
		// we are only generating traffic
		if nGens == 1 {
			// only one generator -> assign entire DDR_A
			nt.gens[genIds[0]].ringBuffAddr = ADDR_DDR_A
			nt.gens[genIds[0]].ringBuffAddrRange = ADDR_RANGE_DDR_A
		} else if nGens == 2 {
			// one generator gets DDR_A, the other one DDR_B
			nt.gens[genIds[0]].ringBuffAddr = ADDR_DDR_A
			nt.gens[genIds[0]].ringBuffAddrRange = ADDR_RANGE_DDR_A
			nt.gens[genIds[1]].ringBuffAddr = ADDR_DDR_B
			nt.gens[genIds[1]].ringBuffAddrRange = ADDR_RANGE_DDR_B
		} else if nGens == 3 {
			// first two generators share DDR_A, third one gets DDR_B
			nt.gens[genIds[0]].ringBuffAddr = ADDR_DDR_A
			nt.gens[genIds[0]].ringBuffAddrRange =
				uint32((uint64(ADDR_RANGE_DDR_A)+1)/2 - 1)
			nt.gens[genIds[1]].ringBuffAddr =
				ADDR_DDR_A + (uint64(ADDR_RANGE_DDR_A)+1)/2
			nt.gens[genIds[1]].ringBuffAddrRange =
				uint32((uint64(ADDR_RANGE_DDR_A)+1)/2 - 1)
			nt.gens[genIds[2]].ringBuffAddr = ADDR_DDR_B
			nt.gens[genIds[2]].ringBuffAddrRange = ADDR_RANGE_DDR_B
		} else if nGens == 4 {
			// first two generators share DDR_A, third and fourth share DDR_B
			nt.gens[genIds[0]].ringBuffAddr = ADDR_DDR_A
			nt.gens[genIds[0]].ringBuffAddrRange =
				uint32((uint64(ADDR_RANGE_DDR_A)+1)/2 - 1)
			nt.gens[genIds[1]].ringBuffAddr =
				ADDR_DDR_A + (uint64(ADDR_RANGE_DDR_A)+1)/2
			nt.gens[genIds[1]].ringBuffAddrRange =
				uint32((uint64(ADDR_RANGE_DDR_A)+1)/2 - 1)
			nt.gens[genIds[2]].ringBuffAddr = ADDR_DDR_B
			nt.gens[genIds[2]].ringBuffAddrRange =
				uint32((uint64(ADDR_RANGE_DDR_B)+1)/2 - 1)
			nt.gens[genIds[3]].ringBuffAddr =
				ADDR_DDR_B + (uint64(ADDR_RANGE_DDR_B)+1)/2
			nt.gens[genIds[3]].ringBuffAddrRange =
				uint32((uint64(ADDR_RANGE_DDR_B)+1)/2 - 1)
		}
	} else if nGens == 0 {
		// we only capture traffic
		if nRecvs == 1 {
			// only one receiver -> assign entire DDR_A
			nt.recvs[recvIds[0]].ringBuffAddr = ADDR_DDR_A
			nt.recvs[recvIds[0]].ringBuffAddrRange = ADDR_RANGE_DDR_A
		} else if nRecvs == 2 {
			// one receiver gets DDR_A, the other one DDR_B
			nt.recvs[recvIds[0]].ringBuffAddr = ADDR_DDR_A
			nt.recvs[recvIds[0]].ringBuffAddrRange = ADDR_RANGE_DDR_A
			nt.recvs[recvIds[1]].ringBuffAddr = ADDR_DDR_B
			nt.recvs[recvIds[1]].ringBuffAddrRange = ADDR_RANGE_DDR_B
		} else if nRecvs == 3 {
			// first two receivers share DDR_A, third one gets DDR_B
			nt.recvs[recvIds[0]].ringBuffAddr = ADDR_DDR_A
			nt.recvs[recvIds[0]].ringBuffAddrRange =
				uint32((uint64(ADDR_RANGE_DDR_A)+1)/2 - 1)
			nt.recvs[recvIds[1]].ringBuffAddr =
				ADDR_DDR_A + (uint64(ADDR_RANGE_DDR_A)+1)/2
			nt.recvs[recvIds[1]].ringBuffAddrRange =
				uint32((uint64(ADDR_RANGE_DDR_A)+1)/2 - 1)
			nt.recvs[recvIds[2]].ringBuffAddr = ADDR_DDR_B
			nt.recvs[recvIds[2]].ringBuffAddrRange = ADDR_RANGE_DDR_B
		} else if nRecvs == 4 {
			// first two receivers share DDR_A, third and fourth share DDR_B
			nt.recvs[recvIds[0]].ringBuffAddr = ADDR_DDR_A
			nt.recvs[recvIds[0]].ringBuffAddrRange =
				uint32((uint64(ADDR_RANGE_DDR_A)+1)/2 - 1)
			nt.recvs[recvIds[1]].ringBuffAddr =
				ADDR_DDR_A + (uint64(ADDR_RANGE_DDR_A)+1)/2
			nt.recvs[recvIds[1]].ringBuffAddrRange =
				uint32((uint64(ADDR_RANGE_DDR_A)+1)/2 - 1)
			nt.recvs[recvIds[2]].ringBuffAddr = ADDR_DDR_B
			nt.recvs[recvIds[2]].ringBuffAddrRange =
				uint32((uint64(ADDR_RANGE_DDR_B)+1)/2 - 1)
			nt.recvs[recvIds[3]].ringBuffAddr =
				ADDR_DDR_B + (uint64(ADDR_RANGE_DDR_B)+1)/2
			nt.recvs[recvIds[3]].ringBuffAddrRange =
				uint32((uint64(ADDR_RANGE_DDR_B)+1)/2 - 1)
		}
	} else {
		// we are generating and capturing
		if nGens == 1 && nRecvs == 1 {
			// generator gets DDR_A, receiver gets DDR_B
			nt.gens[genIds[0]].ringBuffAddr = ADDR_DDR_A
			nt.gens[genIds[0]].ringBuffAddrRange = ADDR_RANGE_DDR_A
			nt.recvs[recvIds[0]].ringBuffAddr = ADDR_DDR_B
			nt.recvs[recvIds[0]].ringBuffAddrRange = ADDR_RANGE_DDR_B
		} else {
			// each generator gets 1 Gbyte for generation in DDR_A, each
			// receiver gets 1 Gbyte for capture in DDR_B
			for i := 0; i < nGens; i++ {
				nt.gens[genIds[i]].ringBuffAddr =
					ADDR_DDR_A + uint64(i*(1024*1024*1024))
				nt.gens[genIds[i]].ringBuffAddrRange = (1024 * 1024 * 1024) - 1
			}
			for i := 0; i < nRecvs; i++ {
				nt.recvs[recvIds[i]].ringBuffAddr =
					ADDR_DDR_B + uint64(i*(1024*1024*1024))
				nt.recvs[recvIds[i]].ringBuffAddrRange =
					(1024 * 1024 * 1024) - 1
			}
		}
	}
}

// configHardware triggers the hardware core configuration.
func (nt *NetworkTester) configHardware() {
	// write generator configuration to hardware
	nt.gens.configHardware()

	// write receiver configuration to hardware
	nt.recvs.configHardware()

	// write timestamp configuration to hardware
	nt.timestamp.configHardware()
}

// resetHardware triggers a reset of all hardware cores. The reset does not
// affect configuration registers.
func (nt *NetworkTester) resetHardware() {
	// reset generators
	nt.gens.resetHardware()

	// reset receivers
	nt.recvs.resetHardware()

	// reset interfaces
	nt.ifaces.resetHardware()

	// disable rate control modules in case they are still active after an
	// erroneous  measurement
	nt.gens.stopRateCtrl(nt.pcieBAR)

	// trigger global hardware reset
	nt.pcieBAR.Write(ADDR_BASE_NT_CTRL+CPUREG_OFFSET_NT_CTRL_RST, 0x1)
	nt.pcieBAR.Write(ADDR_BASE_NT_CTRL+CPUREG_OFFSET_NT_CTRL_RST, 0x0)
}

// checkVersion ensures that the software version matches the hardware version
// of the network tester. It returns an error and aborts the application if a
// mismatch was detected.
func (nt *NetworkTester) checkVersion() {
	ident := nt.pcieBAR.Read(ADDR_BASE_NT_IDENT + CPUREG_OFFSET_NT_IDENT_IDENT)

	hwCRC16 := (ident >> 16) & 0xFFFF
	hwVersion := ident & 0xFFFF

	if hwCRC16 != HW_CRC16 {
		Log(LOG_ERR, "Hardware CRC16 is 0x%04x, expected 0x%04x",
			hwCRC16, HW_CRC16)
	}

	if hwVersion != HW_VERSION {
		Log(LOG_ERR, "Hardware version is 0x%04x, expected 0x%04x",
			hwVersion, HW_VERSION)
	}

	Log(LOG_DEBUG, "Network tester hardware version: 0x%04x", hwVersion)
}

// printDatarates periodically prints out RX and TX data rates of all network
// interfaces. Expects data rate sampling period/print out frequency as
// parameter.
func (nt *NetworkTester) printDatarates(sampleInterval time.Duration) {
	defer nt.syncPrintDatarate.Done()

	// get interfaces
	ifaces := nt.GetInterfaces()

	var stop bool
	for {
		select {
		case _ = <-nt.stopPrintDatarate:
			// goroutine stop requested
			stop = true
		default:
		}

		if stop {
			break
		}

		// iterate over interfaces and print out their rx and tx data rates
		for _, iface := range ifaces {
			datarateTX, datarateTXRaw := iface.GetDatrateTX()
			datarateRX, datarateRXRaw := iface.GetDatrateRX()
			Log(LOG_INFO, "Datarate IF%d: %.3f/%.3f (TX Nom/Raw), %.3f/%.3f (RX Nom/Raw)",
				iface.id, datarateTX, datarateTXRaw, datarateRX, datarateRXRaw)
		}
		Log(LOG_INFO, "----------------------------------------------------------------")

		// wait until hardware data rate counters are updated again
		time.Sleep(sampleInterval)
	}
}
