// +build linux

package main

import (
	"os"
	"golang.org/x/sys/unix"
)

func __watchdog_open(watchdogFile string) (*os.File, error) {
	if _, err := os.Stat(watchdogFile); err != nil {
		return nil, err
	}

	return os.Open(watchdogFile)
}

func (w *watchdog) __ping() {
	unix.IoctlSetInt(int(w.watchdogFile.Fd()), unix.WDIOC_KEEPALIVE, 0)
}

func (w *watchdog) __close(deactivate bool) error {
	w.watchdogFile.Write([]byte("V"))
	w.watchdogFile.Close()
}
