package main

import (
	"io"
	"time"
	"fmt"
	"errors"
	"math"
	"github.com/tarm/serial"
	"github.com/labstack/gommon/log"
	"strings"
	"path/filepath"
	"os"
)

func prepareCommunicator(ttyFile string, baudRate int, dataBits byte,
	stopBits serial.StopBits, parity serial.Parity) *communicator {

	options := &serial.Config{
		Baud:        baudRate,
		Size:        dataBits,
		StopBits:    stopBits,
		Parity:      parity,
		ReadTimeout: time.Millisecond * 100,
	}

	fans := make([]*Fan, 6)
	for i := 0; i < 6; i++ {
		fans[i] = &Fan{
			Id: byte(i + 1),
		}
	}

	return &communicator{
		ttyFile:   ttyFile,
		options:   options,
		pFans:     fans,
		channel:   make(chan comm_msg),
		ticker:    time.NewTicker(time.Second * 1),
		notifiers: make([]updateNotifier, 0),
	}
}

type updateNotifier func(fans []*Fan)

type communicator struct {
	ttyFile   string
	options   *serial.Config
	port      io.ReadWriteCloser
	pFans     []*Fan
	channel   chan comm_msg
	ticker    *time.Ticker
	notifiers []updateNotifier
}

type comm_msg_type int

const (
	comm_msg_type_read  comm_msg_type = 1
	comm_msg_type_write comm_msg_type = 2
	comm_msg_type_close comm_msg_type = 3
)

type comm_msg struct {
	msgType comm_msg_type
	fanId   int
	value   int
}

func (c *communicator) start() {
	go func() {
		for {
			msg := <-c.channel
			switch msg.msgType {
			case comm_msg_type_read:
				c.updateFans()

			case comm_msg_type_write:
				c.__changeSpeed(msg.fanId, msg.value)

			case comm_msg_type_close:
				return
			}
		}
	}()

	c.setSpeed(-1, 0)

	go func() {
		for range c.ticker.C {
			c.channel <- comm_msg{
				msgType: comm_msg_type_read,
				fanId:   -1,
			}
		}
	}()
}

func (c *communicator) write(data []byte, expectedBytes, timeoutMillis int) ([]byte, error) {
	if c.port == nil {
		if err := c.init(); err != nil {
			panic(err)
		}
	}

	if _, err := c.port.Write(data); err != nil {
		return nil, err
	}

	timeout := (time.Millisecond * time.Duration(timeoutMillis)).Nanoseconds()
	deadline := int64(time.Now().Nanosecond()) + timeout

	buf := make([]byte, expectedBytes)
	read := 0

	for {
		buffer := make([]byte, 128)
		n, err := c.port.Read(buffer)
		if err != nil && err != io.EOF {
			return nil, err
		}

		if n > 0 {
			copy(buf[read:], buffer[:n])
			read += n

			if read >= expectedBytes {
				break
			}
		}

		if int64(time.Now().Nanosecond()) > deadline {
			c.port.Close()
			c.port = nil
			return nil, errors.New("update deadline reached, connection closed")
		}
	}

	return buf, nil
}

func (c *communicator) safeWrite(data []byte, expectedBytes, timeoutMillis int) ([]byte, error) {
	for i := 0; i < 5; i++ {
		response, err := c.write(data, expectedBytes, timeoutMillis)
		if err != nil {
			log.Warn(err)
			c.port.Close()
			c.port = nil
			time.Sleep(time.Millisecond * 500)
			continue
		}
		return response, nil
	}
	return nil, errors.New("couldn't communicate to Grid+")
}

func (c *communicator) stop() {
	c.ticker.Stop()
	c.channel <- comm_msg{
		msgType: comm_msg_type_close,
	}
	c.port.Close()
}

func (c *communicator) searchPortByPattern() {
	if !strings.Contains(c.ttyFile, "*") {
		c.options.Name = c.ttyFile

	} else {
		dir, err := os.Open(filepath.Dir(c.ttyFile))
		if err != nil {
			panic(err)
		}

		pattern := strings.Replace(c.ttyFile, "*", "", -1)

		names, err := dir.Readdirnames(-1)
		for _, name := range names {
			name := filepath.Join(filepath.Dir(c.ttyFile), name)
			if strings.HasPrefix(name, pattern) {
				c.options.Name = name
				return
			}
		}
		c.options.Name = c.ttyFile
	}
}
func (c *communicator) connect() error {
	fmt.Printf("Fanboy: Opening communicator with serial port %s... ", c.ttyFile)
	for i := 0; i < 20; i++ {
		c.searchPortByPattern()
		fmt.Printf("interface: %s... ", c.options.Name)
		port, err := serial.OpenPort(c.options)
		if err != nil {
			log.Warn(err)
			fmt.Print(".")
			time.Sleep(time.Second * 5)
			continue
		}
		c.port = port
		break
	}
	fmt.Println("done.")
	return nil
}

func (c *communicator) init() error {
	if err := c.connect(); err != nil {
		return err
	}

	fmt.Print("Fanboy: Initialize Grid+ communication...")
	for {
		response, _ := c.write([]byte{0xC0}, 1, 1000)
		if response[0] == 0x21 {
			break
		}
		time.Sleep(time.Millisecond * 50)
		fmt.Print(".")
	}
	fmt.Println(" done.")
	return nil
}

func (c *communicator) updateFans() {
	//fmt.Print("Fanboy: Triggered fans update... ")
	for i := 0; i < 6; i++ {
		c.pFans[i].update(c)
	}
	//fmt.Println("done.")
	for _, notifier := range c.notifiers {
		notifier(c.pFans)
	}
}

func (c *communicator) setSpeed(id, percentage int) {
	c.channel <- comm_msg{
		msgType: comm_msg_type_write,
		fanId:   id,
		value:   percentage,
	}
}

func (c *communicator) __changeSpeed(id, percentage int) {
	if id > 0 {
		fmt.Printf("Fanboy: Triggered Fan %d speed update %d%%... ", id, percentage)

		c.pFans[id].speed(percentage, c)
		fmt.Println("done.")
		return
	}

	//fmt.Printf("Fanboy: Triggered fans speed update %d%%... ", percentage)
	for i := 0; i < 6; i++ {
		//fmt.Printf("Fan %d... ", i+1)
		c.pFans[i].speed(percentage, c)
	}
	//fmt.Println("done.")
}

func (c *communicator) fans() []*Fan {
	return c.pFans
}

type Fan struct {
	Id      byte    `json:"id"`
	Rpm     uint    `json:"rpm"`
	Volts   float32 `json:"volts"`
	Amps    float32 `json:"amps"`
	Wattage float32 `json:"wattage"`
}

func (f *Fan) update(communicator *communicator) {
	//fmt.Printf("Fan %d", f.Id)
	data := []byte{0x84, f.Id}
	response, err := communicator.safeWrite(data, 5, 500)
	if err != nil {
		panic(err)
	}
	if response[0] != 0xC0 {
		log.Warn(errors.New("invalid response"))
	}
	f.Volts = float32(response[3]) + (0.1 * float32(response[4]))
	//fmt.Print(".")

	data[0] = 0x85
	response, err = communicator.safeWrite(data, 5, 500)
	if err != nil {
		panic(err)
	}
	if response[0] != 0xC0 {
		log.Warn(errors.New("invalid response"))
	}
	f.Amps = float32(response[3]) + (0.1 * float32(response[4]))
	f.Wattage = f.Volts * f.Amps
	//fmt.Print(".")

	data[0] = 0x8A
	response, err = communicator.safeWrite(data, 5, 500)
	if err != nil {
		panic(err)
	}
	if response[0] != 0xC0 {
		log.Warn(errors.New("invalid response"))
	}
	f.Rpm = (uint(response[3]) * 256) + uint(response[4])
	//fmt.Print(". ")
}

func (f *Fan) speed(speed int, communicator *communicator) bool {
	volts := calcVolts(speed)
	data := []byte{0x44, f.Id, 0xC0, 0x00, 0x00, volts[0], volts[1]}
	response, err := communicator.safeWrite(data, 1, 500)
	if err != nil {
		panic(err)
	}
	return response[0] == 0x01
}

func calcVolts(speed int) []byte {
	volts := float64(speed) / 100.0 * 12.0
	inc, dec := math.Modf(volts)
	v1 := byte(inc)
	v2 := byte(dec / 10.0)
	return []byte{v1, v2}
}
