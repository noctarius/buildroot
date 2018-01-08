package main

import (
	"fmt"
	"os"
	"time"
	"github.com/labstack/gommon/log"
)

func prepareWatchdog(watchdogFile string) *watchdog {
	file, err := __watchdog_open(watchdogFile)
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
	w.__ping()
	go func() {
		for {
			msg := <-w.pingChannel
			switch msg {
			case "fin":
				w.__close(true)
				fmt.Println("Fanboy: Hardware watchdog closed")
				return

			case "ping":
				w.__ping()
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
