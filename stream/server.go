package stream

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
)

// Server serves torrent files over HTTP with range-request support.
// The active file can be swapped at runtime for sequential episode playback.
type Server struct {
	mu       sync.RWMutex
	file     *torrent.File
	listener net.Listener
	srv      *http.Server
}

// NewServer creates a streaming HTTP server bound to a random localhost port.
func NewServer() (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	s := &Server{
		listener: ln,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/stream", s.handleStream)

	s.srv = &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 0, // unlimited for streaming
	}

	return s, nil
}

// SetFile swaps the active torrent file being served.
func (s *Server) SetFile(f *torrent.File) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.file = f
}

// URL returns the stream endpoint URL.
func (s *Server) URL() string {
	return fmt.Sprintf("http://%s/stream", s.listener.Addr().String())
}

// Serve starts the HTTP server (blocks until closed).
func (s *Server) Serve() error {
	return s.srv.Serve(s.listener)
}

// Close shuts down the HTTP server.
func (s *Server) Close() error {
	return s.srv.Close()
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	f := s.file
	s.mu.RUnlock()

	if f == nil {
		http.Error(w, "no file loaded", http.StatusServiceUnavailable)
		return
	}

	reader := f.NewReader()
	defer reader.Close()

	// Readahead: 5% of file or 8 MB, whichever is larger.
	readahead := f.Length() / 20
	if readahead < 8*1024*1024 {
		readahead = 8 * 1024 * 1024
	}
	if readahead > f.Length() {
		readahead = f.Length()
	}
	reader.SetReadahead(readahead)
	reader.SetResponsive()

	http.ServeContent(w, r, f.DisplayPath(), time.Time{}, reader)
}
