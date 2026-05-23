package session

import (
	"sync"
	"time"

	"chat2responses/internal/model"
)

const sessionTTL = 30 * time.Minute

// Store maintains per-response-id message history for conversation continuity.
// Each response_id acts as a session key, allowing multiple independent
// conversation chains (e.g. Codex CLI vs Codex Desktop) to coexist.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*session
}

type session struct {
	Messages  []model.ChatMessage
	CreatedAt time.Time
}

func NewStore() *Store {
	s := &Store{
		sessions: make(map[string]*session),
	}
	go s.cleanupLoop()
	return s
}

// Get retrieves the full message history for a response ID chain.
// Returns nil if the session doesn't exist or has expired.
func (s *Store) Get(respID string) []model.ChatMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if sess, ok := s.sessions[respID]; ok {
		return sess.Messages
	}
	return nil
}

// Set stores the full message history for a response ID.
// history should contain all messages up to and including the latest assistant response.
func (s *Store) Set(respID string, history []model.ChatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[respID] = &session{
		Messages:  history,
		CreatedAt: time.Now(),
	}
}

func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, sess := range s.sessions {
			if now.Sub(sess.CreatedAt) > sessionTTL {
				delete(s.sessions, id)
			}
		}
		s.mu.Unlock()
	}
}
