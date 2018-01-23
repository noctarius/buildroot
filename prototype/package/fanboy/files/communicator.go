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
	"encoding/hex"
	"container/list"
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
		notifiers: list.New(),
	}
}

type updateNotifier func(fans []*Fan)

type communicator struct {
	ttyFile    string
	options    *serial.Config
	port       io.ReadWriteCloser
	pFans      []*Fan
	channel    chan comm_msg
	ticker     *time.Ticker
	notifiers  *list.List
	lastUpdate int64
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

func (c *communicator) fans() []*Fan {
	return c.pFans
}

func (c *communicator) addListener(notifier updateNotifier) *list.Element {
	return c.notifiers.PushBack(notifier)
}

func (c *communicator) removeListener(registration *list.Element) {
	c.notifiers.Remove(registration)
}

func (c *communicator) start() {
	go func() {
		for {
			msg := <-c.channel
			switch msg.msgType {
			case comm_msg_type_read:
				c.__updateFans()

			case comm_msg_type_write:
				c.__changeSpeed(msg.fanId, msg.value)

			case comm_msg_type_close:
				return
			}
		}
	}()

	c.__sync()
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

func (c *communicator) stop() {
	c.ticker.Stop()
	c.channel <- comm_msg{
		msgType: comm_msg_type_close,
	}
	c.port.Close()
}

func (c *communicator) __write(data []byte, expectedBytes, timeoutMillis int) ([]byte, error) {
	if c.port == nil {
		if err := c.__resync(); err != nil {
			panic(err)
		}
	}

	if _, err := c.port.Write(data); err != nil {
		return nil, err
	}

	return c.__read(expectedBytes, timeoutMillis)
}

func (c *communicator) __read(expectedBytes, timeoutMillis int) ([]byte, error) {
	timeout := (time.Millisecond * time.Duration(timeoutMillis)).Nanoseconds()
	deadline := int64(time.Now().UnixNano()) + timeout

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

		if int64(time.Now().UnixNano()) > deadline {
			return nil, errors.New("update deadline reached, connection closed")
		}
	}

	return buf, nil
}

func (c *communicator) __safeWrite(data []byte, expectedBytes, timeoutMillis int) ([]byte, error) {
	for i := 0; i < 5; i++ {
		response, err := c.__write(data, expectedBytes, timeoutMillis)
		if err != nil {
			log.Warn(err)
			log.Warnf("Encoded data written to connection: %s", hex.EncodeToString(data))
			c.__resync()
			time.Sleep(time.Millisecond * 500)
			continue
		}
		return response, nil
	}
	return nil, errors.New("couldn't communicate to Grid+")
}

func (c *communicator) __searchPortByPattern() {
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
func (c *communicator) __connect() error {
	fmt.Printf("Fanboy: Opening communicator with serial port %s... ", c.ttyFile)
	if err := c.__connect0(); err != nil {
		return err
	}
	fmt.Println("done.")
	return nil
}

func (c *communicator) __connect0() error {
	for i := 0; i < 20; i++ {
		c.__searchPortByPattern()
		// fmt.Printf("interface: %s... ", c.options.Name)
		port, err := serial.OpenPort(c.options)
		if err != nil {
			if i == 19 {
				return err
			}

			log.Warn(err)
			fmt.Print(".")
			time.Sleep(time.Second * 5)
			continue
		}
		c.port = port
		break
	}
	return nil
}

func (c *communicator) __sync() error {
	fmt.Print("Fanboy: Initialize Grid+ communication...")
	c.__resync()
	fmt.Println(" done.")
	return nil
}

func (c *communicator) __resync() error {
	retry := 0
	for {
		if c.port == nil {
			if err := c.__connect0(); err != nil {
				return err
			}
		}

		response, err := c.__write([]byte{0xC0}, 1, 200)
		if err != nil {
			time.Sleep(time.Millisecond * 50)
			if retry == 10 {
				fmt.Print(":")
				c.__close0()
				retry = 0
			} else {
				fmt.Print(".")
			}
			retry++
			continue
		}

		if response[0] == 0x21 {
			break
		} else if response[0] == 0x2 {
			// reconnect please
			c.__close0()
		} else {
			fmt.Printf("0x%X", response[0])
		}
		retry++
	}
	time.Sleep(time.Millisecond * 50)
	fmt.Println("*")

	// clean inbound queue
	for i := 0; i < 5; i++ {
		c.__read(0, 100)
	}
	return nil
}

func (c *communicator) __close() {
	// fmt.Println(string(debug.Stack()))
	fmt.Print("Fanboy: Closing Grid+ connection... ")
	c.__close0()
	fmt.Println("done.")
}

func (c *communicator) __close0() {
	c.port.Close()
	c.port = nil
}

func (c *communicator) __updateFans() {
	// Prevent overloading the Grid+ right after resyncing
	currentTime := time.Now().UnixNano()
	if currentTime-c.lastUpdate < time.Second.Nanoseconds() {
		return
	}
	c.lastUpdate = currentTime

	for i := 0; i < 6; i++ {
		c.pFans[i].__update(c)
	}
	for e := c.notifiers.Front(); e != nil; e = e.Next() {
		notifier := e.Value.(updateNotifier)
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

		c.pFans[id].__speed(percentage, c)
		fmt.Println("done.")
		return
	}

	for i := 0; i < 6; i++ {
		c.pFans[i].__speed(percentage, c)
	}
}

type Fan struct {
	Id      byte    `json:"id"`
	Rpm     uint    `json:"rpm"`
	Volts   float32 `json:"volts"`
	Amps    float32 `json:"amps"`
	Wattage float32 `json:"wattage"`
}

func (f *Fan) __update(communicator *communicator) {
	f.__updateVolts(communicator)
	f.__updateAmps(communicator)
	f.__updateRpm(communicator)
	f.Wattage = f.Volts * f.Amps
}

func (f *Fan) __updateVolts(communicator *communicator) {
	for {
		data := []byte{0x84, f.Id}
		response, err := communicator.__safeWrite(data, 5, 1000)
		if err != nil {
			panic(err)
		}
		if response[0] != 0xC0 || response[1] != 0x00 || response[2] != 0x00 {
			log.Warn(errors.New("invalid response for volts request"))
			communicator.__resync()
			continue
		}
		f.Volts = float32(response[3]) + (0.1 * float32(response[4]))
		break
	}
}

func (f *Fan) __updateAmps(communicator *communicator) {
	for {
		data := []byte{0x85, f.Id}
		response, err := communicator.__safeWrite(data, 5, 1000)
		if err != nil {
			panic(err)
		}
		if response[0] != 0xC0 || response[1] != 0x00 || response[2] != 0x00 {
			log.Warn(errors.New("invalid response for amps request"))
			communicator.__resync()
			continue
		}
		f.Amps = float32(response[3]) + (0.1 * float32(response[4]))
		break
	}
}

func (f *Fan) __updateRpm(communicator *communicator) {
	for {
		data := []byte{0x8A, f.Id}
		response, err := communicator.__safeWrite(data, 5, 1000)
		if err != nil {
			panic(err)
		}
		if response[0] != 0xC0 || response[1] != 0x00 || response[2] != 0x00 {
			log.Warn(errors.New("invalid response for rpm request"))
			communicator.__resync()
			continue
		}
		f.Rpm = (uint(response[3]) * 256) + uint(response[4])
		break
	}
}

func (f *Fan) __speed(speed int, communicator *communicator) bool {
	volts := f.__calcVolts(speed)
	data := []byte{0x44, f.Id, 0xC0, 0x00, 0x00, volts[0], volts[1]}
	response, err := communicator.__safeWrite(data, 1, 1000)
	if err != nil {
		panic(err)
	}
	return response[0] == 0x01
}

func (f *Fan) __calcVolts(speed int) []byte {
	volts := float64(speed) / 100.0 * 12.0
	inc, dec := math.Modf(volts)
	v1 := byte(inc)
	v2 := byte(dec / 10.0)
	return []byte{v1, v2}
}
