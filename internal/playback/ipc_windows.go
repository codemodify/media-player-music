//go:build windows

package playback

import (
	"net"
	"path/filepath"
	"time"

	"github.com/Microsoft/go-winio"
)

// go-winio supplies Go OS pipe I/O only; no playback libraries are linked.
func ipcAddress(dir string) string     { return `\\.\pipe\` + filepath.Base(dir) }
func restrictIPC(address string) error { return nil }
func dialIPC(address string, timeout time.Duration) (net.Conn, error) {
	return winio.DialPipe(address, &timeout)
}
