package player

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// MPV controls an mpv process via its JSON IPC protocol.
type MPV struct {
	cmd      *exec.Cmd
	sockPath string
	conn     net.Conn
	mu       sync.Mutex
	reqID    int
}

// Launch starts mpv with IPC socket and the given stream URL.
// title is shown in mpv's OSD/window title.
func Launch(url, title string) (*MPV, error) {
	mpvPath, err := exec.LookPath("mpv")
	if err != nil {
		return nil, fmt.Errorf("mpv not found in PATH: %w", err)
	}

	sockPath := filepath.Join(os.TempDir(), fmt.Sprintf("just-stream-mpv-%d.sock", os.Getpid()))
	_ = os.Remove(sockPath) // clean stale socket

	m := &MPV{sockPath: sockPath}

	args := []string{
		"--no-terminal",
		"--force-seekable=yes",
		"--keep-open=yes",
		fmt.Sprintf("--input-ipc-server=%s", sockPath),
		fmt.Sprintf("--force-media-title=%s", title),
		url,
	}

	m.cmd = exec.Command(mpvPath, args...)
	m.cmd.Stdout = os.Stdout
	m.cmd.Stderr = os.Stderr

	if err := m.cmd.Start(); err != nil {
		return nil, fmt.Errorf("start mpv: %w", err)
	}

	// Wait for IPC socket to be available.
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		conn, err := net.Dial("unix", sockPath)
		if err == nil {
			m.conn = conn
			// Drain the initial event messages in background.
			go m.drain()
			return m, nil
		}
	}

	return m, nil // IPC not available, but mpv is running
}

// drain reads and discards incoming messages from mpv IPC socket.
// This prevents the socket buffer from filling up.
func (m *MPV) drain() {
	buf := make([]byte, 4096)
	for {
		if m.conn == nil {
			return
		}
		_, err := m.conn.Read(buf)
		if err != nil {
			return
		}
	}
}

// sendCommand sends a JSON IPC command to mpv.
func (m *MPV) sendCommand(args ...interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn == nil {
		return fmt.Errorf("no IPC connection")
	}

	m.reqID++
	cmd := map[string]interface{}{
		"command":    args,
		"request_id": m.reqID,
	}
	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = m.conn.Write(data)
	return err
}

// LoadFile tells mpv to load a new file URL, replacing the current one.
func (m *MPV) LoadFile(url, title string) error {
	if err := m.sendCommand("loadfile", url, "replace"); err != nil {
		return err
	}
	// Update the media title.
	return m.sendCommand("set_property", "force-media-title", title)
}

// Wait blocks until the mpv process exits.
func (m *MPV) Wait() error {
	return m.cmd.Wait()
}

// Kill terminates the mpv process.
func (m *MPV) Kill() {
	if m.conn != nil {
		// Try graceful quit first.
		_ = m.sendCommand("quit")
		time.Sleep(200 * time.Millisecond)
	}
	if m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
	}
	m.cleanup()
}

// cleanup removes the IPC socket file.
func (m *MPV) cleanup() {
	if m.conn != nil {
		_ = m.conn.Close()
		m.conn = nil
	}
	_ = os.Remove(m.sockPath)
}

// Process returns the underlying os.Process.
func (m *MPV) Process() *os.Process {
	return m.cmd.Process
}
