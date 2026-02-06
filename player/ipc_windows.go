//go:build windows

package player

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Microsoft/go-winio"
)

// ipcPath returns a Windows named pipe path.
func ipcPath() string {
	return fmt.Sprintf(`\\.\pipe\just-stream-mpv-%d`, os.Getpid())
}

// ipcDial connects to a Windows named pipe.
func ipcDial(addr string) (io.ReadWriteCloser, error) {
	timeout := 2 * time.Second
	return winio.DialPipe(addr, &timeout)
}

// ipcPreClean is a no-op on Windows; named pipes are kernel objects.
func ipcPreClean(_ string) {}

// ipcPostClean is a no-op on Windows; the pipe disappears when mpv exits.
func ipcPostClean(_ string) {}
