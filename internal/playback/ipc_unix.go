//go:build !windows

package playback

import (
	"net"
	"os"
	"path/filepath"
	"time"
)

func ipcAddress(dir string) string     { return filepath.Join(dir, "ipc.sock") }
func restrictIPC(address string) error { return os.Chmod(address, 0600) }
func dialIPC(address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", address, timeout)
}
