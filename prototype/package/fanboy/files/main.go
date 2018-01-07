package main

import (
	"fmt"
	"gopkg.in/ini.v1"
	"context"
	"time"
	"github.com/tarm/serial"
)

const (
	default_tty_file        = "/dev/ttyACM*"
	default_tty_baud_rate   = 4800
	default_tty_data_bits   = 8
	default_tty_stop_bits   = 1
	default_tty_parity_mode = serial.ParityNone

	default_watchdog_file = "/dev/watchdog0"

	default_server_port = 80
	default_server_static_path = "/var/fanboy"
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
	baudRate := key.MustInt(default_tty_baud_rate)

	key = section.Key("data_bits")
	dataBits := byte(key.MustInt(default_tty_data_bits))

	key = section.Key("stop_bits")
	stopBits := serial.StopBits(key.MustInt(default_tty_stop_bits))

	fmt.Println("done.")

	communicator := prepareCommunicator(ttyFile, baudRate, dataBits, stopBits, default_tty_parity_mode)

	fmt.Print("Fanboy: Preparing webserver... ")
	section, err = config.NewSection("server")

	key = section.Key("port")
	port := key.MustInt(default_server_port)

	key = section.Key("static_path")
	staticPath := key.MustString(default_server_static_path)

	server := prepareServer(port, staticPath, communicator)
	fmt.Println("done.")

	fmt.Print("Fanboy: Serving api... ")
	quit := make(chan string)
	go server.start(quit)
	fmt.Println("done.")

	communicator.start()

	<-quit

	fmt.Println("Fanboy: Going down master :)")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	fmt.Print("Fanboy: Stoping communicator... ")
	communicator.stop()
	fmt.Println("done.")

	fmt.Print("Fanboy: Stopping server...")
	server.stop(ctx)
	<-ctx.Done()
	fmt.Println("done.")

	fmt.Print("Fanboy: Stoping watchdog... ")
	watchdog.stop()
	fmt.Println("done.")
}
