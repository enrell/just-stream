//go:build !windows

package player

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
)

// ipcPath returns a Unix domain socket path in the OS temp directory.
func ipcPath() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("just-stream-mpv-%d.sock", os.Getpid()))
}

// ipcDial connects to a Unix domain socket.
func ipcDial(addr string) (io.ReadWriteCloser, error) {
	return net.Dial("unix", addr)
}

// ipcPreClean removes a stale socket file before launching mpv.
func ipcPreClean(addr string) {
	_ = os.Remove(addr)
}

// ipcPostClean removes the socket file after mpv exits.
func ipcPostClean(addr string) {
	_ = os.Remove(addr)
}
