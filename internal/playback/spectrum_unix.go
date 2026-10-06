//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package playback

import (
	"os"
	"syscall"
)

func openSpectrumFIFO(path string) (*os.File, error) {
	if err := syscall.Mkfifo(path, 0600); err != nil {
		return nil, err
	}
	// Holding both ends open avoids deadlocks when mpv creates or replaces
	// the filter. Closing this Go file still interrupts its blocked reader.
	return os.OpenFile(path, os.O_RDWR, 0600)
}
