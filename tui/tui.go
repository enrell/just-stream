package tui

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kokoro/just-stream/player"
	memstorage "github.com/kokoro/just-stream/storage"
	"github.com/kokoro/just-stream/stream"
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
)

// --- Messages ---

type (
	metadataReadyMsg struct {
		client *torrent.Client
		t      *torrent.Torrent
	}
	metadataErrMsg  struct{ err error }
	mpvExitedMsg    struct{ err error }
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
}

func NewModel(memStore *memstorage.MemoryStorage, magnetURI string) Model {
	ti := textinput.New()
	ti.Placeholder = "magnet:?xt=urn:btih:..."
	ti.CharLimit = 4096
	ti.Width = 80
	ti.Focus()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6AC1"))

	return Model{
		screen:        screenInput,
		textInput:     ti,
		spinner:       s,
		memStore:      memStore,
		initialMagnet: magnetURI,
		shared:        &shared{},
	}
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
	b.WriteString(helpStyle.Render("enter: submit  esc/ctrl+c: quit"))
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
			return m.beginPlayback(m.cursor, false)
		case "a":
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
	b.WriteString(helpStyle.Render("j/k: navigate  enter: play  a: stream all  q: quit"))
	return b.String()
}

// ──────────────────────────────────────────────
// Playback Screen
// ──────────────────────────────────────────────

func (m Model) updatePlaying(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case mpvExitedMsg:
		if m.streamAll && m.currentFile < m.totalFiles-1 {
			m.freeEpisodeRAM(m.currentFile)
			m.currentFile++
			return m, tea.Batch(
				m.cmdPlayFile(m.currentFile),
				m.cmdTick(),
			)
		}
		// Done: return to file list.
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
		case "n":
			if m.streamAll && m.currentFile < m.totalFiles-1 {
				m.killMPV()
				return m, nil
			}
		case "p":
			if m.streamAll && m.currentFile > 0 {
				m.killMPV()
				m.currentFile -= 2 // mpvExitedMsg will increment
				return m, nil
			}
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
		b.WriteString(helpStyle.Render("n: next  p: previous  q: quit"))
	} else {
		b.WriteString(helpStyle.Render("q: back to list"))
	}
	return b.String()
}

// ──────────────────────────────────────────────
// Commands (run in background goroutines)
// ──────────────────────────────────────────────

func (m Model) cmdFetchMetadata() tea.Cmd {
	memStore := m.memStore
	uri := m.magnetURI
	return func() tea.Msg {
		cfg := torrent.NewDefaultClientConfig()
		cfg.DefaultStorage = memStore
		cfg.ListenPort = 0

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

func (m Model) cmdPlayFile(idx int) tea.Cmd {
	sh := m.shared
	t := m.torrent
	files := m.files

	return func() tea.Msg {
		f := files[idx]
		name := shortName(f.DisplayPath())
		sh.setPlayingName(name)

		// Prioritize selected file; deprioritize others.
		for _, of := range files {
			if of == f {
				of.SetPriority(torrent.PiecePriorityNormal)
			} else {
				of.SetPriority(torrent.PiecePriorityNone)
			}
		}

		// Boost first 5% for fast startup.
		first := f.BeginPieceIndex()
		end := f.EndPieceIndex()
		boost := first + (end-first)/20
		if boost <= first {
			boost = first + 1
		}
		for i := first; i < boost; i++ {
			t.Piece(i).SetPriority(torrent.PiecePriorityNow)
		}

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
		sh.server.SetFile(f)
		streamURL := sh.server.URL()
		sh.mu.Unlock()

		// Launch mpv (new process per episode for clean state).
		sh.mu.Lock()
		if sh.mpv != nil {
			sh.mpv.Kill()
			sh.mpv = nil
		}
		sh.mu.Unlock()

		mpvInst, err := player.Launch(streamURL, name)
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
		m.cmdPlayFile(fileIdx),
		m.cmdTick(),
	)
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

func (m *Model) killMPV() {
	m.shared.mu.Lock()
	defer m.shared.mu.Unlock()
	if m.shared.mpv != nil {
		m.shared.mpv.Kill()
		m.shared.mpv = nil
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
