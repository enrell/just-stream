package stream

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
)

// Server serves torrent files over HTTP with range-request support.
// Each file is available at /stream/<index> for mpv playlist integration.
type Server struct {
	mu       sync.RWMutex
	files    []*torrent.File
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
	mux.HandleFunc("/stream/", s.handleStream)

	s.srv = &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 0, // unlimited for streaming
	}

	return s, nil
}

// SetFiles sets all the torrent files available for streaming.
func (s *Server) SetFiles(files []*torrent.File) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files = files
}

// FileURL returns the stream URL for a specific file index.
func (s *Server) FileURL(idx int) string {
	return fmt.Sprintf("http://%s/stream/%d", s.listener.Addr().String(), idx)
}

// Addr returns the listener address.
func (s *Server) Addr() string {
	return s.listener.Addr().String()
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
	// Parse file index from /stream/<idx>
	idxStr := strings.TrimPrefix(r.URL.Path, "/stream/")
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		http.Error(w, "invalid file index", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	if idx < 0 || idx >= len(s.files) {
		s.mu.RUnlock()
		http.Error(w, "file index out of range", http.StatusNotFound)
		return
	}
	f := s.files[idx]
	s.mu.RUnlock()

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
