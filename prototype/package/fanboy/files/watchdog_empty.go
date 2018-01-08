package main

import (
	"os"
)

func __watchdog_open(watchdogFile string) (*os.File, error) {
	// ignore
	return nil, nil
}

func (w *watchdog) __ping() {
	// ignore
}

func (w *watchdog) __close(deactivate bool) error {
	// ignore
	return nil
}
