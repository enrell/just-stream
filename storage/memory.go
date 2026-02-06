package storage

import (
	"context"
	"io"
	"sync"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

// MemoryStorage implements storage.ClientImpl, storing all piece data in RAM.
type MemoryStorage struct {
	mu       sync.Mutex
	torrents map[metainfo.Hash]*MemTorrent
}

func NewMemory() *MemoryStorage {
	return &MemoryStorage{
		torrents: make(map[metainfo.Hash]*MemTorrent),
	}
}

func (ms *MemoryStorage) OpenTorrent(_ context.Context, info *metainfo.Info, infoHash metainfo.Hash) (storage.TorrentImpl, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	t := &MemTorrent{
		pieces:    make(map[int]*memPiece),
		pieceLen:  info.PieceLength,
		numPieces: info.NumPieces(),
		info:      info,
	}
	ms.torrents[infoHash] = t
	return storage.TorrentImpl{
		Piece: t.Piece,
		Close: t.Close,
	}, nil
}

// GetTorrent returns the in-memory torrent storage for the given hash.
func (ms *MemoryStorage) GetTorrent(h metainfo.Hash) *MemTorrent {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.torrents[h]
}

func (ms *MemoryStorage) Close() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.torrents = nil
	return nil
}

// MemTorrent holds all in-memory pieces for one torrent.
type MemTorrent struct {
	mu        sync.Mutex
	pieces    map[int]*memPiece
	pieceLen  int64
	numPieces int
	info      *metainfo.Info
}

func (mt *MemTorrent) Piece(p metainfo.Piece) storage.PieceImpl {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	idx := p.Index()
	if mp, ok := mt.pieces[idx]; ok {
		return mp
	}

	length := p.Length()
	mp := &memPiece{
		data: make([]byte, length),
		len:  length,
	}
	mt.pieces[idx] = mp
	return mp
}

// FreePieces releases memory for the given piece range [start, end).
// Used to reclaim RAM after an episode finishes playing.
func (mt *MemTorrent) FreePieces(start, end int) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	for i := start; i < end; i++ {
		delete(mt.pieces, i)
	}
}

func (mt *MemTorrent) Close() error {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.pieces = nil
	return nil
}

// memPiece stores one piece's data in a byte slice.
type memPiece struct {
	mu       sync.RWMutex
	data     []byte
	len      int64
	complete bool
}

func (mp *memPiece) ReadAt(p []byte, off int64) (int, error) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	if off >= int64(len(mp.data)) {
		return 0, io.EOF
	}
	n := copy(p, mp.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (mp *memPiece) WriteAt(p []byte, off int64) (int, error) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	end := off + int64(len(p))
	if end > int64(len(mp.data)) {
		grown := make([]byte, end)
		copy(grown, mp.data)
		mp.data = grown
	}
	n := copy(mp.data[off:], p)
	return n, nil
}

func (mp *memPiece) MarkComplete() error {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.complete = true
	return nil
}

func (mp *memPiece) MarkNotComplete() error {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.complete = false
	return nil
}

func (mp *memPiece) Completion() storage.Completion {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return storage.Completion{
		Complete: mp.complete,
		Ok:       true,
	}
}
