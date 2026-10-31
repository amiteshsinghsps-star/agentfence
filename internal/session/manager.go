package session

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// State represents the circuit-breaker state of an agent session.
type State string

const (
	StateOpen       State = "OPEN"
	StateHalfOpen   State = "HALF_OPEN"
	StateFenced     State = "FENCED"
	StateTerminated State = "TERMINATED"
)

// Session represents a single agent session with its state, intent, and metadata.
type Session struct {
	ID                string    `json:"session_id"`
	AgentID           string    `json:"agent_id"`
	ParentSessionID   string    `json:"parent_session_id,omitempty"`
	DeclaredIntent    string    `json:"declared_intent"`
	DeclaredScopeTags []string  `json:"declared_scope_tags"`
	state             State
	SuspicionScore    float64   `json:"suspicion_score"`
	CallCount         int       `json:"call_count"`
	StartedAt         time.Time `json:"started_at"`
	EndedAt           *time.Time `json:"ended_at,omitempty"`
	mu                sync.RWMutex
}

// CurrentState returns the session's current state with proper locking.
func (s *Session) CurrentState() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Snapshot returns a thread-safe copy of the session state for policy evaluation.
func (s *Session) Snapshot() SessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return SessionSnapshot{
		SessionID:     s.ID,
		AgentID:       s.AgentID,
		DeclaredScope: s.DeclaredScopeTags,
		State:         string(s.state),
		CallCount:     s.CallCount,
	}
}

// SessionSnapshot is a thread-safe, immutable view of a session for policy input.
type SessionSnapshot struct {
	SessionID     string   `json:"session_id"`
	AgentID       string   `json:"agent_id"`
	DeclaredScope []string `json:"declared_scope"`
	State         string   `json:"state"`
	CallCount     int      `json:"call_count"`
}

// Manager handles creation, lookup, and state transitions for agent sessions.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewManager creates a new session manager.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// CreateSession creates a new agent session with the given parameters.
func (m *Manager) CreateSession(ctx context.Context, agentID, parentSessionID, declaredIntent string, declaredScope []string) (*Session, error) {
	sess := &Session{
		ID:                newSessionID(),
		AgentID:           agentID,
		ParentSessionID:   parentSessionID,
		DeclaredIntent:    declaredIntent,
		DeclaredScopeTags: declaredScope,
		state:             StateOpen,
		SuspicionScore:    0,
		CallCount:         0,
		StartedAt:         time.Now(),
	}

	m.mu.Lock()
	m.sessions[sess.ID] = sess
	m.mu.Unlock()

	return sess, nil
}

// GetSession retrieves a session by ID.
func (m *Manager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	return sess, nil
}

// SetState atomically transitions a session's state.
func (m *Manager) SetState(ctx context.Context, sessionID string, newState State) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	sess.mu.Lock()
	sess.state = newState
	if newState == StateTerminated {
		now := time.Now()
		sess.EndedAt = &now
	}
	sess.mu.Unlock()

	return nil
}

// SetStateString sets the session state from a string value.
func (m *Manager) SetStateString(ctx context.Context, sessionID, newState string) error {
	return m.SetState(ctx, sessionID, State(newState))
}

// GetState retrieves the current state of a session.
func (m *Manager) GetState(ctx context.Context, sessionID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		return "", fmt.Errorf("session %s not found", sessionID)
	}

	sess.mu.RLock()
	defer sess.mu.RUnlock()
	return string(sess.state), nil
}

// IncrementCallCount atomically increments the call count for a session.
func (m *Manager) IncrementCallCount(ctx context.Context, sessionID string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	sess.mu.Lock()
	sess.CallCount++
	sess.mu.Unlock()

	return nil
}

// UpdateSuspicionScore atomically updates the suspicion score for a session.
func (m *Manager) UpdateSuspicionScore(ctx context.Context, sessionID string, score float64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	sess.mu.Lock()
	sess.SuspicionScore = score
	sess.mu.Unlock()

	return nil
}

// ListSessions returns all sessions, optionally filtered by state.
func (m *Manager) ListSessions(ctx context.Context, stateFilter ...State) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Session
	for _, sess := range m.sessions {
		if len(stateFilter) > 0 {
			currentState := sess.CurrentState()
			match := false
			for _, f := range stateFilter {
				if currentState == f {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		result = append(result, sess)
	}
	return result
}

// TerminateSession ends a session and records the end time.
func (m *Manager) TerminateSession(ctx context.Context, sessionID string) error {
	return m.SetState(ctx, sessionID, StateTerminated)
}

// newSessionID generates a unique session ID using crypto/rand.
func newSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
