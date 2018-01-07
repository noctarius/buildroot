package main

import (
	"fmt"
	"os"
	"time"
	"golang.org/x/sys/unix"
	"github.com/labstack/gommon/log"
)

func prepareWatchdog(watchdogFile string) *watchdog {
	if _, err := os.Stat(watchdogFile); err != nil {
		panic(err)
	}

	file, err := os.Open(watchdogFile)
	if err != nil {
		panic(err)
	}

	return &watchdog{
		watchdogFile: file,
		pingChannel:  make(chan string),
		ticker:       time.NewTicker(time.Second * 5),
	}
}

type watchdog struct {
	watchdogFile *os.File
	pingChannel  chan string
	ticker       *time.Ticker
}

func (w *watchdog) start() {
	closeData := []byte("V")
	unix.IoctlSetInt(int(w.watchdogFile.Fd()), unix.WDIOC_KEEPALIVE, 0)
	go func() {
		for {
			msg := <-w.pingChannel
			switch msg {
			case "fin":
				w.watchdogFile.Write(closeData)
				w.watchdogFile.Close()
				fmt.Println("Fanboy: Hardware watchdog closed")
				return

			case "ping":
				unix.IoctlSetInt(int(w.watchdogFile.Fd()), unix.WDIOC_KEEPALIVE, 0)
				log.Debug("Fanboy: Hardware watchdog pinged")
			}
		}
	}()

	go func() {
		for range w.ticker.C {
			w.pingChannel <- "ping"
		}
	}()
}

func (w *watchdog) stop() {
	w.ticker.Stop()
	w.pingChannel <- "fin"
}
