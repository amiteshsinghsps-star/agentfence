package session

import (
	"context"
	"testing"
)

func TestCreateSession(t *testing.T) {
	m := NewManager()
	sess, err := m.CreateSession(context.Background(), "agent-1", "", "read-only logs", []string{"read-only"})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if sess.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if sess.CurrentState() != StateOpen {
		t.Errorf("expected state OPEN, got %s", sess.CurrentState())
	}
	if sess.CallCount != 0 {
		t.Errorf("expected CallCount 0, got %d", sess.CallCount)
	}
}

func TestGetSession(t *testing.T) {
	m := NewManager()
	created, _ := m.CreateSession(context.Background(), "agent-1", "", "test", nil)
	got, err := m.GetSession(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("expected session %s, got %s", created.ID, got.ID)
	}
}

func TestGetSessionNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetSession(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestSetState(t *testing.T) {
	m := NewManager()
	sess, _ := m.CreateSession(context.Background(), "agent-1", "", "test", nil)

	if err := m.SetState(context.Background(), sess.ID, StateFenced); err != nil {
		t.Fatalf("SetState failed: %v", err)
	}

	updated, _ := m.GetSession(context.Background(), sess.ID)
	if updated.CurrentState() != StateFenced {
		t.Errorf("expected state FENCED, got %s", updated.CurrentState())
	}
}

func TestSetStateString(t *testing.T) {
	m := NewManager()
	sess, _ := m.CreateSession(context.Background(), "agent-1", "", "test", nil)

	if err := m.SetStateString(context.Background(), sess.ID, "FENCED"); err != nil {
		t.Fatalf("SetStateString failed: %v", err)
	}

	state, _ := m.GetState(context.Background(), sess.ID)
	if state != "FENCED" {
		t.Errorf("expected state FENCED, got %s", state)
	}
}

func TestGetState(t *testing.T) {
	m := NewManager()
	sess, _ := m.CreateSession(context.Background(), "agent-1", "", "test", nil)

	state, err := m.GetState(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}
	if state != "OPEN" {
		t.Errorf("expected OPEN, got %s", state)
	}
}

func TestListSessionsEmpty(t *testing.T) {
	m := NewManager()
	sessions := m.ListSessions(context.Background())
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSessionsWithFilter(t *testing.T) {
	m := NewManager()
	m.CreateSession(context.Background(), "agent-1", "", "test", nil)
	m.CreateSession(context.Background(), "agent-2", "", "test", nil)

	m.SetState(context.Background(), m.ListSessions(context.Background())[0].ID, StateFenced)

	fenced := m.ListSessions(context.Background(), StateFenced)
	if len(fenced) != 1 {
		t.Errorf("expected 1 fenced session, got %d", len(fenced))
	}
}

func TestIncrementCallCount(t *testing.T) {
	m := NewManager()
	sess, _ := m.CreateSession(context.Background(), "agent-1", "", "test", nil)

	for i := 0; i < 5; i++ {
		m.IncrementCallCount(context.Background(), sess.ID)
	}

	updated, _ := m.GetSession(context.Background(), sess.ID)
	if updated.CallCount != 5 {
		t.Errorf("expected CallCount 5, got %d", updated.CallCount)
	}
}

func TestUpdateSuspicionScore(t *testing.T) {
	m := NewManager()
	sess, _ := m.CreateSession(context.Background(), "agent-1", "", "test", nil)

	m.UpdateSuspicionScore(context.Background(), sess.ID, 0.75)

	updated, _ := m.GetSession(context.Background(), sess.ID)
	if updated.SuspicionScore != 0.75 {
		t.Errorf("expected SuspicionScore 0.75, got %f", updated.SuspicionScore)
	}
}

func TestTerminateSession(t *testing.T) {
	m := NewManager()
	sess, _ := m.CreateSession(context.Background(), "agent-1", "", "test", nil)

	m.TerminateSession(context.Background(), sess.ID)

	if sess.CurrentState() != StateTerminated {
		t.Errorf("expected TERMINATED, got %s", sess.CurrentState())
	}
	if sess.EndedAt == nil {
		t.Error("expected EndedAt to be set")
	}
}

func TestSnapshot(t *testing.T) {
	m := NewManager()
	sess, _ := m.CreateSession(context.Background(), "agent-1", "", "test", []string{"read-only"})

	snap := sess.Snapshot()
	if snap.SessionID != sess.ID {
		t.Errorf("expected session ID %s, got %s", sess.ID, snap.SessionID)
	}
	if len(snap.DeclaredScope) != 1 || snap.DeclaredScope[0] != "read-only" {
		t.Errorf("expected scope [read-only], got %v", snap.DeclaredScope)
	}
	if snap.State != "OPEN" {
		t.Errorf("expected OPEN, got %s", snap.State)
	}
}
