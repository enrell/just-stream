package tui

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/net/proxy"

"github.com/enrell/just-stream/config"
	"github.com/enrell/just-stream/player"
	memstorage "github.com/enrell/just-stream/storage"
	"github.com/enrell/just-stream/stream"
)

// --- Styles ---

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6AC1"))

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9B9B9B"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6AC1")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D4D4D4"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7EC8E3"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF4444")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6AC1")).
			Bold(true)

	progressFullStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF6AC1"))

	progressEmptyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#333333"))

	playingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B")).
			Bold(true)

	seedingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB86C"))
)

// --- Screens ---

type screen int

const (
	screenInput   screen = iota // paste magnet link
	screenLoading               // waiting for metadata
	screenFiles                 // file selection list
	screenPlaying               // playback status
	screenConfig                // settings (mpv path)
)

// --- Messages ---

type (
	metadataReadyMsg struct {
		client *torrent.Client
		t      *torrent.Torrent
	}
	metadataErrMsg  struct{ err error }
	mpvExitedMsg    struct{ err error }
	playlistPosMsg  struct{ pos int }
	configSavedMsg  struct{ err error }
	tickMsg         time.Time
	submitMagnetMsg struct{ uri string }
)

// shared holds mutable state accessed from both the TUI thread and
// background goroutines (commands). This avoids Bubble Tea's value-copy
// problem for fields that need mutation from tea.Cmd goroutines.
type shared struct {
	mu          sync.Mutex
	server      *stream.Server
	mpv         *player.MPV
	client      *torrent.Client
	playingName string
	program     *tea.Program // set after program starts, used for Send()
}

func (s *shared) setPlayingName(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.playingName = name
}

func (s *shared) getPlayingName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.playingName
}

// --- Model ---

type Model struct {
	screen   screen
	width    int
	height   int
	quitting bool
	err      error

	// Input screen
	textInput textinput.Model

	// Loading screen
	spinner   spinner.Model
	magnetURI string

	// File list screen
	torrent     *torrent.Torrent
	files       []*torrent.File
	cursor      int
	torrentName string
	streamAll   bool

	// Playback screen
	memStore    *memstorage.MemoryStorage
	currentFile int
	totalFiles  int
	startTime   time.Time

	// Shared mutable state for background goroutines
	shared *shared

	// Magnet passed as CLI arg
	initialMagnet string

	// Proxy URL string (socks5://host:port or http://host:port)
	proxyURL string

	// Config
	cfg          *config.Config
	configInput  textinput.Model // text input for mpv path on config screen
	prevScreen   screen          // screen to return to after config
	configStatus string          // transient status message on config screen
}

func NewModel(memStore *memstorage.MemoryStorage, magnetURI string, proxyURL string, cfg *config.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "magnet:?xt=urn:btih:..."
	ti.CharLimit = 4096
	ti.Width = 80
	ti.Focus()

	ci := textinput.New()
	ci.Placeholder = "/usr/bin/mpv"
	ci.CharLimit = 512
	ci.Width = 60

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6AC1"))

	if cfg == nil {
		cfg = &config.Config{}
	}

	return Model{
		screen:        screenInput,
		textInput:     ti,
		configInput:   ci,
		spinner:       s,
		memStore:      memStore,
		initialMagnet: magnetURI,
		proxyURL:      proxyURL,
		cfg:           cfg,
		shared:        &shared{},
	}
}

// SetProgram stores the tea.Program reference so background callbacks can
// send messages into the Bubble Tea event loop. This is safe as a value
// receiver because it writes through the shared pointer.
func (m Model) SetProgram(p *tea.Program) {
	m.shared.mu.Lock()
	defer m.shared.mu.Unlock()
	m.shared.program = p
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink}
	if m.initialMagnet != "" {
		cmds = append(cmds, func() tea.Msg {
			return submitMagnetMsg{uri: m.initialMagnet}
		})
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			m.cleanup()
			return m, tea.Quit
		}
		// ctrl+s opens config from any screen except config itself.
		if msg.String() == "ctrl+s" && m.screen != screenConfig {
			m.prevScreen = m.screen
			m.screen = screenConfig
			m.configStatus = ""
			m.configInput.SetValue(m.cfg.MpvPath)
			m.configInput.Focus()
			return m, textinput.Blink
		}
	}

	switch m.screen {
	case screenInput:
		return m.updateInput(msg)
	case screenLoading:
		return m.updateLoading(msg)
	case screenFiles:
		return m.updateFiles(msg)
	case screenPlaying:
		return m.updatePlaying(msg)
	case screenConfig:
		return m.updateConfig(msg)
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	var content string
	switch m.screen {
	case screenInput:
		content = m.viewInput()
	case screenLoading:
		content = m.viewLoading()
	case screenFiles:
		content = m.viewFiles()
	case screenPlaying:
		content = m.viewPlaying()
	case screenConfig:
		content = m.viewConfig()
	}
	return content + "\n"
}

// ──────────────────────────────────────────────
// Input Screen
// ──────────────────────────────────────────────

func (m Model) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case submitMagnetMsg:
		m.magnetURI = msg.uri
		m.screen = screenLoading
		return m, tea.Batch(m.spinner.Tick, m.cmdFetchMetadata())
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			uri := strings.TrimSpace(m.textInput.Value())
			if uri == "" {
				return m, nil
			}
			m.magnetURI = uri
			m.screen = screenLoading
			return m, tea.Batch(m.spinner.Tick, m.cmdFetchMetadata())
		case "esc":
			m.quitting = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) viewInput() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("just-stream"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("Torrent streaming to mpv with Anime4K"))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("Paste a magnet link:"))
	b.WriteString("\n\n")
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("enter: submit  ctrl+s: config  esc/ctrl+c: quit"))
	return b.String()
}

// ──────────────────────────────────────────────
// Loading Screen
// ──────────────────────────────────────────────

func (m Model) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case metadataReadyMsg:
		m.shared.mu.Lock()
		m.shared.client = msg.client
		m.shared.mu.Unlock()

		m.torrent = msg.t
		m.torrentName = msg.t.Name()
		m.files = filterMediaFiles(msg.t.Files())
		if len(m.files) == 0 {
			m.files = msg.t.Files()
		}
		sortFilesByName(m.files)
		m.screen = screenFiles
		return m, nil
	case metadataErrMsg:
		m.err = msg.err
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) viewLoading() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("just-stream"))
	b.WriteString("\n\n")
	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("ctrl+c: quit"))
	} else {
		b.WriteString(m.spinner.View())
		b.WriteString(statusStyle.Render(" Fetching torrent metadata..."))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("Connecting to peers and downloading info"))
	}
	return b.String()
}

// ──────────────────────────────────────────────
// File Selection Screen
// ──────────────────────────────────────────────

func (m Model) updateFiles(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "j", "down":
			if m.cursor < len(m.files)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "g", "home":
			m.cursor = 0
		case "G", "end":
			m.cursor = len(m.files) - 1
			case "enter":
				m.err = nil // Clear previous error
				return m.beginPlayback(m.cursor, false)
			case "a":
				m.err = nil // Clear previous error
				return m.beginPlayback(0, true)
		case "q", "esc":
			m.quitting = true
			m.cleanup()
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) viewFiles() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("just-stream"))
	b.WriteString("\n")
	b.WriteString(headerStyle.Render(m.torrentName))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("%d episodes found", len(m.files))))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n")
	}

	visible := m.height - 10
	if visible < 5 {
		visible = 20
	}

	startIdx := 0
	if m.cursor >= visible {
		startIdx = m.cursor - visible + 1
	}
	endIdx := startIdx + visible
	if endIdx > len(m.files) {
		endIdx = len(m.files)
	}

	for i := startIdx; i < endIdx; i++ {
		f := m.files[i]
		name := shortName(f.DisplayPath())
		size := humanSize(f.Length())

		if i == m.cursor {
			b.WriteString(selectedStyle.Render(fmt.Sprintf("  > [%02d] %s  %s", i+1, name, size)))
		} else {
			line := fmt.Sprintf("    [%02d] %s", i+1, name)
			b.WriteString(normalStyle.Render(line))
			b.WriteString(dimStyle.Render(fmt.Sprintf("  %s", size)))
		}
		b.WriteString("\n")
	}

	if startIdx > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("    ... %d more above", startIdx)))
		b.WriteString("\n")
	}
	if endIdx < len(m.files) {
		b.WriteString(dimStyle.Render(fmt.Sprintf("    ... %d more below", len(m.files)-endIdx)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k: navigate  enter: play  a: stream all  ctrl+s: config  q: quit"))
	return b.String()
}

// ──────────────────────────────────────────────
// Playback Screen
// ──────────────────────────────────────────────

func (m Model) updatePlaying(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case playlistPosMsg:
		newPos := msg.pos
		if newPos < 0 || newPos >= len(m.files) {
			return m, nil
		}

		// Free RAM for old episode if moving forward.
		oldPos := m.currentFile
		if newPos > oldPos {
			for i := oldPos; i < newPos; i++ {
				m.freeEpisodeRAM(i)
			}
		}

		m.currentFile = newPos
		m.shared.setPlayingName(shortName(m.files[newPos].DisplayPath()))

		// Update priorities: boost new file, deprioritize others.
		m.setPriorities(newPos)

		return m, nil

case mpvExitedMsg:
			// mpv exited (user quit or playlist ended). Return to file list.
			if msg.err != nil {
				m.err = fmt.Errorf("mpv failed to start: %w", msg.err)
			}
			m.cleanupPlayback()
			m.screen = screenFiles
			if m.currentFile < len(m.files) {
				m.cursor = m.currentFile
			}
			return m, nil

	case tickMsg:
		return m, m.cmdTick()

	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			m.cleanup()
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) viewPlaying() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("just-stream"))
	b.WriteString("\n\n")

	name := m.shared.getPlayingName()
	if name == "" {
		name = "loading..."
	}

	if m.streamAll {
		b.WriteString(playingStyle.Render(fmt.Sprintf("  Episode %d/%d", m.currentFile+1, m.totalFiles)))
		b.WriteString("\n")
	}
	b.WriteString(normalStyle.Render(fmt.Sprintf("  Playing: %s", name)))
	b.WriteString("\n\n")

	if m.torrent != nil {
		stats := m.torrent.Stats()
		b.WriteString(statusStyle.Render(fmt.Sprintf("  Peers:    %d active / %d total",
			stats.ActivePeers, stats.TotalPeers)))
		b.WriteString("\n")

		if m.currentFile < len(m.files) {
			f := m.files[m.currentFile]
			var completed int64
			total := int64(f.EndPieceIndex() - f.BeginPieceIndex())
			for i := f.BeginPieceIndex(); i < f.EndPieceIndex(); i++ {
				if m.torrent.PieceState(i).Complete {
					completed++
				}
			}
			pct := float64(0)
			if total > 0 {
				pct = float64(completed) / float64(total) * 100
			}

			barW := 40
			filled := int(pct / 100 * float64(barW))
			if filled > barW {
				filled = barW
			}
			bar := progressFullStyle.Render(strings.Repeat("█", filled)) +
				progressEmptyStyle.Render(strings.Repeat("░", barW-filled))

			b.WriteString(normalStyle.Render(fmt.Sprintf("  Buffer:   %s %.1f%%", bar, pct)))
			b.WriteString("\n")

			// Show seeding status
			if pct >= 100 {
				b.WriteString(seedingStyle.Render("  Status:   Seeding (sharing with peers)"))
				b.WriteString("\n")
			}
		}

		elapsed := time.Since(m.startTime).Truncate(time.Second)
		b.WriteString(normalStyle.Render(fmt.Sprintf("  Elapsed:  %s", elapsed)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.streamAll {
		b.WriteString(helpStyle.Render("Shift+>/< in mpv: next/prev  q: quit"))
	} else {
		b.WriteString(helpStyle.Render("q: back to list"))
	}
	return b.String()
}

// ──────────────────────────────────────────────
// Config Screen
// ──────────────────────────────────────────────

func (m Model) updateConfig(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case configSavedMsg:
		if msg.err != nil {
			m.configStatus = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.configStatus = "Saved!"
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			path := strings.TrimSpace(m.configInput.Value())
			m.cfg.MpvPath = path
			return m, m.cmdSaveConfig()
		case "esc":
			m.screen = m.prevScreen
			// Re-focus the magnet input if returning there.
			if m.screen == screenInput {
				m.textInput.Focus()
				return m, textinput.Blink
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.configInput, cmd = m.configInput.Update(msg)
	return m, cmd
}

func (m Model) viewConfig() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("just-stream"))
	b.WriteString(" ")
	b.WriteString(dimStyle.Render("settings"))
	b.WriteString("\n\n")

	b.WriteString(normalStyle.Render("  mpv path (leave empty for auto-detect):"))
	b.WriteString("\n\n")
	b.WriteString("  ")
	b.WriteString(m.configInput.View())
	b.WriteString("\n\n")

	if m.configStatus != "" {
		if strings.HasPrefix(m.configStatus, "Error") {
			b.WriteString("  ")
			b.WriteString(errorStyle.Render(m.configStatus))
		} else {
			b.WriteString("  ")
			b.WriteString(playingStyle.Render(m.configStatus))
		}
		b.WriteString("\n\n")
	}

	cfgPath, _ := config.Path()
	if cfgPath != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  config: %s", cfgPath)))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("enter: save  esc: back  ctrl+c: quit"))
	return b.String()
}

func (m Model) cmdSaveConfig() tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		return configSavedMsg{err: config.Save(cfg)}
	}
}

// ──────────────────────────────────────────────
// Commands (run in background goroutines)
// ──────────────────────────────────────────────

func (m Model) cmdFetchMetadata() tea.Cmd {
	memStore := m.memStore
	uri := m.magnetURI
	proxyURL := m.proxyURL
	return func() tea.Msg {
		cfg := torrent.NewDefaultClientConfig()
		cfg.DefaultStorage = memStore
		cfg.ListenPort = 0

		// Configure proxy if provided.
		if proxyURL != "" {
			if err := configureProxy(cfg, proxyURL); err != nil {
				return metadataErrMsg{err: fmt.Errorf("proxy config: %w", err)}
			}
		}

		client, err := torrent.NewClient(cfg)
		if err != nil {
			return metadataErrMsg{err: fmt.Errorf("create client: %w", err)}
		}

		t, err := client.AddMagnet(uri)
		if err != nil {
			client.Close()
			return metadataErrMsg{err: fmt.Errorf("add magnet: %w", err)}
		}

		<-t.GotInfo()
		return metadataReadyMsg{client: client, t: t}
	}
}

func (m Model) cmdStartPlayback() tea.Cmd {
	sh := m.shared
	t := m.torrent
	files := m.files
	streamAllMode := m.streamAll
	startIdx := m.currentFile
	mpvPath := m.cfg.MpvPath

	return func() tea.Msg {
		// Ensure HTTP server is running.
		sh.mu.Lock()
		if sh.server == nil {
			srv, err := stream.NewServer()
			if err != nil {
				sh.mu.Unlock()
				return mpvExitedMsg{err: err}
			}
			sh.server = srv
			go srv.Serve()
		}
		sh.server.SetFiles(files)
		sh.mu.Unlock()

		// Build URL and title lists.
		var urls []string
		var titles []string

		if streamAllMode {
			// All files as playlist entries.
			for i := range files {
				sh.mu.Lock()
				u := sh.server.FileURL(i)
				sh.mu.Unlock()
				urls = append(urls, u)
				titles = append(titles, shortName(files[i].DisplayPath()))
			}
		} else {
			// Single file.
			sh.mu.Lock()
			u := sh.server.FileURL(startIdx)
			sh.mu.Unlock()
			urls = append(urls, u)
			titles = append(titles, shortName(files[startIdx].DisplayPath()))
		}

		// Set playing name.
		playIdx := 0
		if streamAllMode {
			playIdx = startIdx
		}
		sh.setPlayingName(titles[playIdx])

		// Prioritize starting file, deprioritize others.
		actualIdx := startIdx
		if !streamAllMode {
			actualIdx = startIdx
		}
		for i, f := range files {
			if i == actualIdx {
				f.SetPriority(torrent.PiecePriorityNormal)
			} else {
				f.SetPriority(torrent.PiecePriorityNone)
			}
		}

		// Boost first 5% of starting file for fast startup.
		startFile := files[actualIdx]
		first := startFile.BeginPieceIndex()
		end := startFile.EndPieceIndex()
		boost := first + (end-first)/20
		if boost <= first {
			boost = first + 1
		}
		for i := first; i < boost; i++ {
			t.Piece(i).SetPriority(torrent.PiecePriorityNow)
		}

		// Kill any existing mpv.
		sh.mu.Lock()
		if sh.mpv != nil {
			sh.mpv.Kill()
			sh.mpv = nil
		}
		sh.mu.Unlock()

		// Build mpv launch options with playlist position callback.
		launchStartIdx := 0
		if streamAllMode {
			launchStartIdx = startIdx
		}

		opts := player.LaunchOpts{
			URLs:       urls,
			Titles:     titles,
			StartIndex: launchStartIdx,
			MpvPath:    mpvPath,
			OnPlaylistPos: func(pos int) {
				sh.mu.Lock()
				p := sh.program
				sh.mu.Unlock()
				if p != nil {
					p.Send(playlistPosMsg{pos: pos})
				}
			},
		}

		mpvInst, err := player.Launch(opts)
		if err != nil {
			return mpvExitedMsg{err: err}
		}

		sh.mu.Lock()
		sh.mpv = mpvInst
		sh.mu.Unlock()

		// Block until mpv exits.
		waitErr := mpvInst.Wait()

		sh.mu.Lock()
		sh.mpv = nil
		sh.mu.Unlock()

		return mpvExitedMsg{err: waitErr}
	}
}

func (m Model) cmdTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ──────────────────────────────────────────────
// State transitions & cleanup
// ──────────────────────────────────────────────

func (m Model) beginPlayback(fileIdx int, all bool) (tea.Model, tea.Cmd) {
	m.screen = screenPlaying
	m.currentFile = fileIdx
	m.streamAll = all
	m.startTime = time.Now()
	if all {
		m.totalFiles = len(m.files)
	} else {
		m.totalFiles = fileIdx + 1
	}
	return m, tea.Batch(
		m.cmdStartPlayback(),
		m.cmdTick(),
	)
}

// setPriorities updates torrent piece priorities for the current file.
func (m *Model) setPriorities(fileIdx int) {
	if fileIdx >= len(m.files) {
		return
	}
	for i, f := range m.files {
		if i == fileIdx {
			f.SetPriority(torrent.PiecePriorityNormal)
		} else {
			f.SetPriority(torrent.PiecePriorityNone)
		}
	}

	// Boost first 5% of new file.
	f := m.files[fileIdx]
	first := f.BeginPieceIndex()
	end := f.EndPieceIndex()
	boost := first + (end-first)/20
	if boost <= first {
		boost = first + 1
	}
	for i := first; i < boost; i++ {
		m.torrent.Piece(i).SetPriority(torrent.PiecePriorityNow)
	}
}

func (m *Model) freeEpisodeRAM(fileIdx int) {
	if fileIdx >= len(m.files) {
		return
	}
	f := m.files[fileIdx]
	ih := m.torrent.InfoHash()
	mt := m.memStore.GetTorrent(ih)
	if mt != nil {
		mt.FreePieces(f.BeginPieceIndex(), f.EndPieceIndex())
	}
}

func (m *Model) cleanupPlayback() {
	m.shared.mu.Lock()
	defer m.shared.mu.Unlock()
	if m.shared.mpv != nil {
		m.shared.mpv.Kill()
		m.shared.mpv = nil
	}
	if m.shared.server != nil {
		m.shared.server.Close()
		m.shared.server = nil
	}
}

func (m *Model) cleanup() {
	m.cleanupPlayback()
	m.shared.mu.Lock()
	defer m.shared.mu.Unlock()
	if m.shared.client != nil {
		m.shared.client.Close()
		m.shared.client = nil
	}
}

// ──────────────────────────────────────────────
// Proxy configuration
// ──────────────────────────────────────────────

// configureProxy sets up the torrent client config to route traffic
// through a SOCKS5 or HTTP proxy.
func configureProxy(cfg *torrent.ClientConfig, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse proxy URL: %w", err)
	}

	switch u.Scheme {
	case "socks5", "socks5h":
		// SOCKS5 proxy: route tracker and peer connections through it.
		auth := &proxy.Auth{}
		if u.User != nil {
			auth.User = u.User.Username()
			auth.Password, _ = u.User.Password()
		} else {
			auth = nil
		}

		dialer, err := proxy.SOCKS5("tcp", u.Host, auth, proxy.Direct)
		if err != nil {
			return fmt.Errorf("create SOCKS5 dialer: %w", err)
		}

		ctxDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return fmt.Errorf("SOCKS5 dialer does not support DialContext")
		}

		// Route HTTP tracker announces through SOCKS5.
		cfg.HTTPProxy = http.ProxyURL(u)
		// Route tracker TCP connections through SOCKS5.
		cfg.TrackerDialContext = ctxDialer.DialContext
		// Route webseed HTTP connections through SOCKS5.
		cfg.HTTPDialContext = ctxDialer.DialContext

		// DHT uses UDP which SOCKS5 cannot proxy; disable it.
		cfg.NoDHT = true
		// Disable local peer discovery (not useful through proxy).
		cfg.DisablePEX = true

	case "http", "https":
		// HTTP proxy: only useful for HTTP tracker announces.
		cfg.HTTPProxy = http.ProxyURL(u)
		// Cannot proxy peer TCP connections or DHT through HTTP proxy,
		// but HTTP trackers will be routed through the proxy.

	default:
		return fmt.Errorf("unsupported proxy scheme %q (use socks5:// or http://)", u.Scheme)
	}

	return nil
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

func filterMediaFiles(files []*torrent.File) []*torrent.File {
	exts := map[string]bool{
		".mkv": true, ".mp4": true, ".avi": true, ".webm": true,
		".m4v": true, ".mov": true, ".ts": true, ".flv": true,
		".ogv": true, ".wmv": true,
	}
	var media []*torrent.File
	for _, f := range files {
		path := strings.ToLower(f.DisplayPath())
		for ext := range exts {
			if strings.HasSuffix(path, ext) {
				media = append(media, f)
				break
			}
		}
	}
	return media
}

func sortFilesByName(files []*torrent.File) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].DisplayPath() < files[j].DisplayPath()
	})
}

func shortName(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func humanSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
