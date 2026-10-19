// Package cache holds the assembled ranking snapshot in memory. There is no
// database or disk; the app refetches on startup.
package cache

import (
	"sync"

	"github.com/shanehull/markethunt/internal/model"
)

// Store holds the latest snapshot, guarded by a mutex.
type Store struct {
	mu   sync.RWMutex
	snap *model.Snapshot
}

// New returns an empty store.
func New() *Store { return &Store{} }

// Put replaces the snapshot.
func (s *Store) Put(snap *model.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap = snap
}

// Snapshot returns the latest snapshot, or nil before the first ingest.
func (s *Store) Snapshot() *model.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snap
}
