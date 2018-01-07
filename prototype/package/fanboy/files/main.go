package main

import (
	"fmt"
	"gopkg.in/ini.v1"
	"github.com/jacobsa/go-serial/serial"
)

const (
	default_tty_file        = "/dev/ttyACM0"
	default_tty_baud_rate   = uint(4800)
	default_tty_data_bits   = uint(8)
	default_tty_stop_bits   = uint(1)
	default_tty_parity_mode = serial.PARITY_NONE

	default_watchdog_file = "/dev/watchdog0"

	default_server_port = 80
)

func main() {
	fmt.Print("Fanboy: Loading configuration... ")
	options := ini.LoadOptions{
		AllowBooleanKeys:    true,
		IgnoreInlineComment: true,
		Loose:               true,
	}

	config, err := ini.LoadSources(options, "/etc/fanboy.conf")
	if err != nil {
		panic(err)
	}

	section, err := config.NewSection("watchdog")
	if err != nil {
		panic(err)
	}
	fmt.Println("done.")

	fmt.Print("Fanboy: Readying hardware watchdog... ")
	key := section.Key("file")

	watchdog := prepareWatchdog(key.MustString(default_watchdog_file))
	watchdog.start()
	fmt.Println("done.")

	fmt.Print("Fanboy: Reading serial port configuration... ")
	section, err = config.NewSection("tty")
	if err != nil {
		panic(err)
	}

	key = section.Key("file")
	ttyFile := key.MustString(default_tty_file)

	key = section.Key("baud_rate")
	baudRate := key.MustUint(default_tty_baud_rate)

	key = section.Key("data_bits")
	dataBits := key.MustUint(default_tty_data_bits)

	key = section.Key("stop_bits")
	stopBits := key.MustUint(default_tty_stop_bits)

	fmt.Println("done.")

	fmt.Printf("Fanboy: Opening communicator with serial port %s... ", ttyFile)
	communicator := prepareCommunicator(ttyFile, baudRate, dataBits, stopBits, default_tty_parity_mode)
	fmt.Println("done.")
	communicator.start()

	fmt.Print("Fanboy: Preparing webserver... ")
	section, err = config.NewSection("server")

	key = section.Key("port")
	port := key.MustInt(default_server_port)

	server := prepareServer(port, communicator)
	fmt.Println("done.")

	fmt.Println("Fanboy: Serving api.")
	go server.start()
}
