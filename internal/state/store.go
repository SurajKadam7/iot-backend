package state

import (
	"sync"

	"github.com/google/uuid"
	"github.com/surajkadam7/iot-backend/internal/models"
)

type Store struct {
	mu sync.RWMutex
	m  map[uuid.UUID]models.Reading
}

func New() *Store {
	return &Store{m: make(map[uuid.UUID]models.Reading)}
}

func (s *Store) Set(r models.Reading) {
	s.mu.Lock()
	s.m[r.DeviceID] = r
	s.mu.Unlock()
}

func (s *Store) Get(id uuid.UUID) (models.Reading, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.m[id]
	return r, ok
}

func (s *Store) Delete(id uuid.UUID) {
	s.mu.Lock()
	delete(s.m, id)
	s.mu.Unlock()
}

func (s *Store) Snapshot(deviceIDs []uuid.UUID) []models.Reading {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Reading, 0, len(deviceIDs))
	for _, id := range deviceIDs {
		if r, ok := s.m[id]; ok {
			out = append(out, r)
		}
	}
	return out
}

func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m)
}
