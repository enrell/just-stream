package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// MPV controls an mpv process via its JSON IPC protocol.
type MPV struct {
	cmd     *exec.Cmd
	ipcAddr string // socket path (unix) or pipe name (windows)
	conn    io.ReadWriteCloser
	mu      sync.Mutex
	reqID   int

	// Playlist position tracking
	posMu       sync.Mutex
	playlistPos int
	onPosChange func(pos int) // callback when playlist-pos changes
}

// LaunchOpts configures the mpv launch.
type LaunchOpts struct {
	// URLs is the list of stream URLs to play (playlist entries).
	URLs []string
	// Titles is the list of media titles corresponding to each URL.
	Titles []string
	// StartIndex is the playlist index to start playing from.
	StartIndex int
	// OnPlaylistPos is called when mpv's playlist position changes.
	OnPlaylistPos func(pos int)
	// MpvPath overrides exec.LookPath when non-empty.
	MpvPath string
}

// Launch starts mpv with an IPC endpoint, loading the given URLs as a playlist.
func Launch(opts LaunchOpts) (*MPV, error) {
	mpvPath := opts.MpvPath
	if mpvPath == "" {
		var err error
		mpvPath, err = exec.LookPath("mpv")
		if err != nil {
			// Try mpvnet too (Windows fork with GUI)
			if mpvPath, err = exec.LookPath("mpvnet"); err == nil {
				mpvPath = "mpvnet"
			}
		}
		if mpvPath == "" {
			// Try common Windows paths
			username := os.Getenv("USERNAME")
			if username == "" {
				if home, err := os.UserHomeDir(); err == nil {
					username = filepath.Base(home)
				}
			}
			if username != "" {
				commonPaths := []string{
					`C:\Program Files\mpv\mpv.exe`,
					`C:\Program Files (x86)\mpv\mpv.exe`,
					`C:\tools\mpv\mpv.exe`,
					`C:\Users\` + username + `\scoop\apps\mpv\current\mpv.exe`,
					`C:\Users\` + username + `\scoop\shims\mpv.exe`,
					`C:\Users\` + username + `\AppData\Local\Microsoft\WindowsApps\mpv.exe`,
					`C:\Users\` + username + `\AppData\Local\Programs\mpv.net\mpvnet.exe`,
					`C:\ProgramData\chocolatey\lib\mpv\tools\mpv.exe`,
				}
				for _, path := range commonPaths {
					if _, statErr := os.Stat(path); statErr == nil {
						mpvPath = path
						break
					}
				}
			}
			if mpvPath == "" {
				return nil, fmt.Errorf("mpv not found in PATH or common locations (C:\\Program Files\\mpv, scoop, mpv.net, etc). Please install mpv or set MPV_PATH environment variable")
			}
		}
	}

	addr := ipcPath()
	ipcPreClean(addr)

	m := &MPV{
		ipcAddr:     addr,
		playlistPos: opts.StartIndex,
		onPosChange: opts.OnPlaylistPos,
	}

	args := []string{
		"--no-terminal",
		"--force-seekable=yes",
		fmt.Sprintf("--input-ipc-server=%s", addr),
	}

	// First URL goes as a direct argument, rest are appended via IPC.
	if len(opts.URLs) > 0 {
		if opts.StartIndex < len(opts.Titles) && opts.Titles[opts.StartIndex] != "" {
			args = append(args, fmt.Sprintf("--force-media-title=%s", opts.Titles[opts.StartIndex]))
		}
		args = append(args, opts.URLs[0])
	}

	m.cmd = exec.Command(mpvPath, args...)
	m.cmd.Stdout = os.Stdout
	m.cmd.Stderr = os.Stderr

	if err := m.cmd.Start(); err != nil {
		return nil, fmt.Errorf("start mpv: %w", err)
	}

	// Poll until the IPC endpoint is ready.
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		conn, err := ipcDial(addr)
		if err == nil {
			m.conn = conn

			if len(opts.URLs) > 1 {
				go m.appendPlaylist(opts)
			} else {
				go m.eventLoop()
			}
			return m, nil
		}
	}

	return m, nil
}

// appendPlaylist adds the remaining URLs to mpv's playlist via IPC,
// then seeks to the correct start position.
func (m *MPV) appendPlaylist(opts LaunchOpts) {
	time.Sleep(200 * time.Millisecond)

	for i := 1; i < len(opts.URLs); i++ {
		_ = m.sendCommand("loadfile", opts.URLs[i], "append")
		if i < len(opts.Titles) && opts.Titles[i] != "" {
			_ = m.sendCommand("set_property",
				fmt.Sprintf("playlist/%d/title", i),
				opts.Titles[i])
		}
	}

	if len(opts.Titles) > 0 && opts.Titles[0] != "" {
		_ = m.sendCommand("set_property", "playlist/0/title", opts.Titles[0])
	}

	if opts.StartIndex > 0 && opts.StartIndex < len(opts.URLs) {
		_ = m.sendCommand("set_property", "playlist-pos", opts.StartIndex)
	}

	_ = m.sendCommand("observe_property", 1, "playlist-pos")
	m.eventLoop()
}

// eventLoop reads IPC messages from mpv and dispatches events.
func (m *MPV) eventLoop() {
	if m.conn == nil {
		return
	}

	_ = m.sendCommand("observe_property", 1, "playlist-pos")

	scanner := bufio.NewScanner(m.conn)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var msg map[string]interface{}
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		if event, ok := msg["event"].(string); ok && event == "property-change" {
			name, _ := msg["name"].(string)
			if name == "playlist-pos" {
				if data, ok := msg["data"].(float64); ok {
					pos := int(data)
					m.posMu.Lock()
					m.playlistPos = pos
					cb := m.onPosChange
					m.posMu.Unlock()
					if cb != nil {
						cb(pos)
					}
				}
			}
		}
	}
}

// PlaylistPos returns the current playlist position.
func (m *MPV) PlaylistPos() int {
	m.posMu.Lock()
	defer m.posMu.Unlock()
	return m.playlistPos
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

	if wc, ok := m.conn.(interface{ SetWriteDeadline(time.Time) error }); ok {
		_ = wc.SetWriteDeadline(time.Now().Add(2 * time.Second))
	}
	_, err = m.conn.Write(data)
	return err
}

// SetMediaTitle updates the force-media-title property.
func (m *MPV) SetMediaTitle(title string) error {
	return m.sendCommand("set_property", "force-media-title", title)
}

// Wait blocks until the mpv process exits.
func (m *MPV) Wait() error {
	err := m.cmd.Wait()
	m.cleanup()
	return err
}

// Kill terminates the mpv process.
func (m *MPV) Kill() {
	if m.conn != nil {
		_ = m.sendCommand("quit")
		done := make(chan struct{})
		go func() {
			_ = m.cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			if m.cmd.Process != nil {
				_ = m.cmd.Process.Kill()
			}
		}
	} else if m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
	}
	m.cleanup()
}

func (m *MPV) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.conn != nil {
		_ = m.conn.Close()
		m.conn = nil
	}
	ipcPostClean(m.ipcAddr)
}
