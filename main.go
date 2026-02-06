package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kokoro/just-stream/config"
	memstorage "github.com/kokoro/just-stream/storage"
	"github.com/kokoro/just-stream/tui"
)

func main() {
	proxyFlag := flag.String("proxy", "", "proxy URL (socks5://host:port or http://host:port)")
	flag.StringVar(proxyFlag, "x", "", "proxy URL (shorthand for -proxy)")
	flag.Parse()

	// Accept magnet link as positional argument to skip the input screen.
	var magnetURI string
	if flag.NArg() > 0 {
		magnetURI = flag.Arg(0)
	}

	// Also respect ALL_PROXY / all_proxy env var as fallback.
	proxyURL := *proxyFlag
	if proxyURL == "" {
		proxyURL = os.Getenv("ALL_PROXY")
	}
	if proxyURL == "" {
		proxyURL = os.Getenv("all_proxy")
	}

	// Load persisted config (mpv path, etc).
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", err)
		cfg = &config.Config{}
	}

	memStore := memstorage.NewMemory()

	model := tui.NewModel(memStore, magnetURI, proxyURL, cfg)

	// Give the model access to the program so background callbacks
	// (e.g. mpv playlist-pos changes) can send messages.
	// SetProgram writes to the shared pointer, which all Model value-copies
	// share, so this works with Bubble Tea's value-copy update pattern.
	p := tea.NewProgram(model, tea.WithAltScreen())
	model.SetProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
