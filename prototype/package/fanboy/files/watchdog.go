package main

import (
	"fmt"
	"os"
	"time"
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
	pingData := []byte("P")
	closeData := []byte("V")
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
				w.watchdogFile.Write(pingData)
				fmt.Println("Fanboy: Hardware watchdog pinged")
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
