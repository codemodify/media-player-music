//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package playback

import (
	"errors"
	"os"
)

func openSpectrumFIFO(string) (*os.File, error) {
	return nil, errors.New("visualizer metadata FIFO is unsupported on this platform")
}
