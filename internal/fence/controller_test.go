package fence

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

type mockSessionStore struct {
	state map[string]string
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{state: make(map[string]string)}
}

func (m *mockSessionStore) SetState(ctx context.Context, sessionID, state string) error {
	m.state[sessionID] = state
	return nil
}

func (m *mockSessionStore) GetState(ctx context.Context, sessionID string) (string, error) {
	if s, ok := m.state[sessionID]; ok {
		return s, nil
	}
	return "OPEN", nil
}

type mockEventPublisher struct {
	events []string
}

func (m *mockEventPublisher) PublishFence(sessionID string, level Level, trigger, reason string, triggeredAt time.Time) error {
	m.events = append(m.events, string(level))
	return nil
}

type mockOrchestrator struct {
	isolated map[string]bool
	killed   map[string]bool
}

func newMockOrchestrator() *mockOrchestrator {
	return &mockOrchestrator{
		isolated: make(map[string]bool),
		killed:   make(map[string]bool),
	}
}

func (m *mockOrchestrator) KillSession(ctx context.Context, sessionID string) error {
	m.killed[sessionID] = true
	return nil
}

func (m *mockOrchestrator) IsolateNetwork(ctx context.Context, sessionID string) error {
	m.isolated[sessionID] = true
	return nil
}

func TestEvaluateScore_BelowThreshold(t *testing.T) {
	ctrl := NewController(
		&mockOrchestrator{}, newMockSessionStore(), &mockEventPublisher{},
		0.50, 0.75, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
	)
	err := ctrl.EvaluateScore(context.Background(), "sess-1", 0.1)
	if err != nil {
		t.Errorf("expected no error for score below threshold, got %v", err)
	}
}

func TestEvaluateScore_FreezeAtThreshold(t *testing.T) {
	store := newMockSessionStore()
	pub := &mockEventPublisher{}
	ctrl := NewController(
		newMockOrchestrator(), store, pub,
		0.50, 0.75, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
	)

	err := ctrl.EvaluateScore(context.Background(), "sess-1", 0.60)
	if err != nil {
		t.Fatalf("EvaluateScore failed: %v", err)
	}

	state, _ := store.GetState(context.Background(), "sess-1")
	if state != "FENCED" {
		t.Errorf("expected FENCED, got %s", state)
	}
}

func TestEvaluateScore_QuarantineAtThreshold(t *testing.T) {
	store := newMockSessionStore()
	pub := &mockEventPublisher{}
	ctrl := NewController(
		newMockOrchestrator(), store, pub,
		0.50, 0.75, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
	)

	err := ctrl.EvaluateScore(context.Background(), "sess-1", 0.85)
	if err != nil {
		t.Fatalf("EvaluateScore failed: %v", err)
	}

	state, _ := store.GetState(context.Background(), "sess-1")
	if state != "FENCED" {
		t.Errorf("expected FENCED, got %s", state)
	}
}

func TestFence_Freeze(t *testing.T) {
	store := newMockSessionStore()
	orch := newMockOrchestrator()
	pub := &mockEventPublisher{}
	ctrl := NewController(orch, store, pub, 0.50, 0.75, nil)

	err := ctrl.Fence(context.Background(), "sess-1", LevelFreeze, "test", "testing freeze")
	if err != nil {
		t.Fatalf("Fence failed: %v", err)
	}

	state, _ := store.GetState(context.Background(), "sess-1")
	if state != "FENCED" {
		t.Errorf("expected FENCED state, got %s", state)
	}

	// Freeze should NOT call KillSession
	if orch.killed["sess-1"] {
		t.Error("FREEZE should not call KillSession")
	}
}

func TestFence_Kill(t *testing.T) {
	store := newMockSessionStore()
	orch := newMockOrchestrator()
	pub := &mockEventPublisher{}
	ctrl := NewController(orch, store, pub, 0.50, 0.75, nil)

	err := ctrl.Fence(context.Background(), "sess-1", LevelKill, "test", "testing kill")
	if err != nil {
		t.Fatalf("Fence failed: %v", err)
	}

	if !orch.isolated["sess-1"] {
		t.Error("KILL should call IsolateNetwork")
	}
	if !orch.killed["sess-1"] {
		t.Error("KILL should call KillSession")
	}
}

func TestFence_AlreadyFenced(t *testing.T) {
	store := newMockSessionStore()
	store.SetState(context.Background(), "sess-1", "FENCED")
	ctrl := NewController(newMockOrchestrator(), store, &mockEventPublisher{}, 0.50, 0.75, nil)

	// Should be no-op (not escalate to KILL)
	err := ctrl.Fence(context.Background(), "sess-1", LevelFreeze, "test", "already fenced")
	if err != nil {
		t.Errorf("expected no error for already fenced, got %v", err)
	}
}

func TestFence_EscalateToKill(t *testing.T) {
	store := newMockSessionStore()
	store.SetState(context.Background(), "sess-1", "FENCED")
	orch := newMockOrchestrator()
	ctrl := NewController(orch, store, &mockEventPublisher{}, 0.50, 0.75, nil)

	err := ctrl.Fence(context.Background(), "sess-1", LevelKill, "test", "escalating to kill")
	if err != nil {
		t.Errorf("expected no error escalating to kill, got %v", err)
	}

	if !orch.isolated["sess-1"] {
		t.Error("escalation to KILL should call IsolateNetwork")
	}
}

func TestUnfence(t *testing.T) {
	store := newMockSessionStore()
	store.SetState(context.Background(), "sess-1", "FENCED")
	ctrl := NewController(newMockOrchestrator(), store, &mockEventPublisher{}, 0.50, 0.75, nil)

	err := ctrl.Unfence(context.Background(), "sess-1", "test complete", "operator-1")
	if err != nil {
		t.Fatalf("Unfence failed: %v", err)
	}

	state, _ := store.GetState(context.Background(), "sess-1")
	if state != "OPEN" {
		t.Errorf("expected OPEN after unfence, got %s", state)
	}
}

func TestDefaultThresholds(t *testing.T) {
	freeze, quarantine := DefaultThresholds()
	if freeze != 0.50 {
		t.Errorf("expected freeze 0.50, got %f", freeze)
	}
	if quarantine != 0.75 {
		t.Errorf("expected quarantine 0.75, got %f", quarantine)
	}
}

func TestEvaluateScore_ExactBoundary(t *testing.T) {
	store := newMockSessionStore()
	ctrl := NewController(
		newMockOrchestrator(), store, &mockEventPublisher{},
		0.50, 0.75, nil,
	)

	tests := []struct {
		score float64
		want  string // empty = no fence
	}{
		{0.49, ""},
		{0.50, "FENCED"}, // freeze threshold
		{0.74, "FENCED"},
		{0.75, "FENCED"}, // quarantine threshold
		{0.99, "FENCED"},
	}

	for _, tt := range tests {
		store.state["test-boundary"] = "OPEN"
		ctrl.EvaluateScore(context.Background(), "test-boundary", tt.score)
		state, _ := store.GetState(context.Background(), "test-boundary")

		if tt.want == "" && state != "OPEN" {
			t.Errorf("score=%.2f: expected OPEN, got %s", tt.score, state)
		}
		if tt.want != "" && state != tt.want {
			t.Errorf("score=%.2f: expected %s, got %s", tt.score, tt.want, state)
		}
	}
}
