package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	memstorage "github.com/kokoro/just-stream/storage"
	"github.com/kokoro/just-stream/tui"
)

func main() {
	// Optional: accept magnet link as argument to skip the input screen.
	var magnetURI string
	if len(os.Args) > 1 {
		magnetURI = os.Args[1]
	}

	memStore := memstorage.NewMemory()

	model := tui.NewModel(memStore, magnetURI)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
