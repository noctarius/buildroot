package main

import (
	"io"
	"github.com/jacobsa/go-serial/serial"
	"time"
	"fmt"
	"errors"
)

func prepareCommunicator(ttyFile string, baudRate, dataBits, stopBits uint,
	parityMode serial.ParityMode) *communicator {

	portOptions := serial.OpenOptions{
		PortName:              ttyFile,
		BaudRate:              baudRate,
		DataBits:              dataBits,
		StopBits:              stopBits,
		ParityMode:            parityMode,
		MinimumReadSize:       1,
		InterCharacterTimeout: 100,
	}

	port, err := serial.Open(portOptions)
	if err != nil {
		panic(err)
	}

	fans := make([]fan, 6)
	for i := 0; i < 5; i++ {
		fans[i] = fan{
			id: byte(i),
		}
	}

	return &communicator{
		port:  port,
		pFans: fans,
	}
}

type communicator struct {
	port    io.ReadWriteCloser
	pFans   []fan
	channel chan string
	ticker  *time.Ticker
}

func (c *communicator) start() {
	c.init()

	c.ticker = time.NewTicker(time.Second)
	go func() {
		for range c.ticker.C {
			c.channel <- "update"
		}
	}()

	go func() {
		for {
			msg := <-c.channel
			switch msg {
			case "update":
				c.updateFans()
			}
		}
	}()
}

func (c *communicator) write(data []byte, expectedBytes, timeoutMillis int) []byte {
	if _, err := c.port.Write(data); err != nil {
		panic(err)
	}

	timeout := (time.Millisecond * time.Duration(timeoutMillis)).Nanoseconds()
	deadline := int64(time.Now().Nanosecond()) + timeout

	buf := make([]byte, expectedBytes)
	read := 0

	for {
		buffer := make([]byte, 128)
		n, err := c.port.Read(buffer)
		if err != nil {
			panic(err)
		}

		copy(buf[read:], buffer[:n])
		read += n

		if read >= expectedBytes {
			break
		}

		if int64(time.Now().Nanosecond()) > deadline {
			panic(errors.New("update deadline reached"))
		}
	}

	return buf
}

func (c *communicator) stop() {
	c.port.Close()
}

func (c *communicator) init() {
	fmt.Print("Fanboy: Initialize Grid+ communication...")
	for {
		response := c.write([]byte{0xC0}, 1, 100)
		if response[0] == 0x21 {
			break
		}
		time.Sleep(time.Millisecond * 50)
		fmt.Print(".")
	}
	fmt.Println(" done.")
}

func (c *communicator) updateFans() {
	for i := 0; i < 5; i++ {
		c.pFans[i].update(c)
	}
}

func (c *communicator) setSpeed(id, percentage int) {
	if id != -1 {
		c.pFans[id].speed(percentage, c)
		return
	}

	for i := 0; i < 5; i++ {
		c.pFans[i].speed(percentage, c)
	}
}

func (c *communicator) fans() []fan {
	return c.pFans
}

type fan struct {
	id      byte    `json:"id"`
	rpm     uint    `json:"rpm"`
	volts   float32 `json:"volts"`
	amps    float32 `json:"amps"`
	wattage float32 `json:"wattage"`
}

func (f *fan) update(communicator *communicator) {
	data := []byte{0x84, f.id}
	response := communicator.write(data, 5, 100)
	if response[0] != 0xC0 {
		panic(errors.New("invalid response"))
	}
	f.volts = float32(response[3]) + (0.1 * float32(response[4]))

	data[0] = 0x85
	response = communicator.write(data, 5, 100)
	if response[0] != 0xC0 {
		panic(errors.New("invalid response"))
	}
	f.amps = float32(response[3]) + (0.1 * float32(response[4]))
	f.wattage = f.volts * f.amps

	data[0] = 0x8A
	response = communicator.write(data, 5, 100)
	if response[0] != 0xC0 {
		panic(errors.New("invalid response"))
	}
	f.rpm = (uint(response[3]) * 256) + uint(response[4])
}

func (f *fan) speed(speed int, communicator *communicator) bool {
	volts := calcVolts(speed)
	data := []byte{44, f.id, 0xC0, 0x00, 0x00, volts[0], volts[1]}
	response := communicator.write(data, 1, 100)
	return response[0] == 0x01
}

func calcVolts(speed int) []byte {
	volts := int((float32(speed) * 12.0) * 100)
	v1 := byte(volts % 100)
	v2 := byte((volts - (int(v1) * 100)) / 100)
	return []byte{v1, v2}
}
