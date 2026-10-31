package propagation

import (
	"testing"
)

func TestEvaluateSpawn_ParentOpen(t *testing.T) {
	b := NewBreaker(false)
	flag, state := b.EvaluateSpawn("parent-1", "OPEN", "read-only logs", []string{"read-only"})
	if flag {
		t.Error("expected flag=false for OPEN parent")
	}
	if state != "OPEN" {
		t.Errorf("expected OPEN, got %s", state)
	}
}

func TestEvaluateSpawn_ParentFenced(t *testing.T) {
	b := NewBreaker(false)
	flag, state := b.EvaluateSpawn("parent-1", "FENCED", "read-only logs", []string{"read-only"})
	if !flag {
		t.Error("expected flag=true for FENCED parent")
	}
	if state != "HALF_OPEN" {
		t.Errorf("expected HALF_OPEN, got %s", state)
	}
}

func TestEvaluateSpawn_ParentHalfOpen(t *testing.T) {
	b := NewBreaker(false)
	flag, state := b.EvaluateSpawn("parent-1", "HALF_OPEN", "read-only logs", []string{"read-only"})
	if flag {
		t.Error("expected flag=false for HALF_OPEN parent with flagOnSpawn=false")
	}
	if state != "HALF_OPEN" {
		t.Errorf("expected HALF_OPEN, got %s", state)
	}
}

func TestEvaluateSpawn_ParentHalfOpenWithFlag(t *testing.T) {
	b := NewBreaker(true)
	flag, state := b.EvaluateSpawn("parent-1", "HALF_OPEN", "read-only logs", []string{"read-only"})
	if !flag {
		t.Error("expected flag=true for HALF_OPEN parent with flagOnSpawn=true")
	}
	if state != "HALF_OPEN" {
		t.Errorf("expected HALF_OPEN, got %s", state)
	}
}

func TestEvaluateSpawn_IntentMismatch(t *testing.T) {
	b := NewBreaker(false)
	flag, state := b.EvaluateSpawn("parent-1", "OPEN",
		"write to production database", []string{"read-only"})
	if !flag {
		t.Error("expected flag=true for intent mismatch")
	}
	if state != "HALF_OPEN" {
		t.Errorf("expected HALF_OPEN, got %s", state)
	}
}

func TestEvaluateSpawn_ParentTerminated(t *testing.T) {
	b := NewBreaker(false)
	flag, state := b.EvaluateSpawn("parent-1", "TERMINATED", "any task", nil)
	if !flag {
		t.Error("expected flag=true for TERMINATED parent")
	}
	if state != "HALF_OPEN" {
		t.Errorf("expected HALF_OPEN, got %s", state)
	}
}

func TestRecordSpawn(t *testing.T) {
	b := NewBreaker(false)
	edge := b.RecordSpawn("parent-1", "child-1", "OPEN", false)
	if edge.ParentSessionID != "parent-1" {
		t.Errorf("expected parent-1, got %s", edge.ParentSessionID)
	}
	if edge.ChildSessionID != "child-1" {
		t.Errorf("expected child-1, got %s", edge.ChildSessionID)
	}
	if edge.Flagged {
		t.Error("expected flagged=false")
	}
}

func TestCheckInheritedInstructionMismatch_WriteUnderReadOnly(t *testing.T) {
	b := NewBreaker(false)
	mismatch := b.CheckInheritedInstructionMismatch("child-1", "kubectl.delete", []string{"read-only"})
	if !mismatch {
		t.Error("expected mismatch for write under read-only parent scope")
	}
}

func TestCheckInheritedInstructionMismatch_ReadOnlyMatch(t *testing.T) {
	b := NewBreaker(false)
	mismatch := b.CheckInheritedInstructionMismatch("child-1", "fs.read", []string{"read-only"})
	if mismatch {
		t.Error("expected no mismatch for read under read-only")
	}
}

func TestGetSpawnTree(t *testing.T) {
	b := NewBreaker(false)
	b.RecordSpawn("p1", "c1", "OPEN", false)
	b.RecordSpawn("p1", "c2", "OPEN", true)

	tree := b.GetSpawnTree()
	if len(tree) != 2 {
		t.Errorf("expected 2 edges, got %d", len(tree))
	}
}

func TestGetChildSessions(t *testing.T) {
	b := NewBreaker(false)
	b.RecordSpawn("parent-1", "child-1", "OPEN", false)
	b.RecordSpawn("parent-1", "child-2", "OPEN", false)
	b.RecordSpawn("parent-2", "child-3", "OPEN", false)

	children := b.GetChildSessions("parent-1")
	if len(children) != 2 {
		t.Errorf("expected 2 children, got %d", len(children))
	}
}

func TestContainsWriteKeyword(t *testing.T) {
	tests := []struct {
		intent string
		want   bool
	}{
		{"read-only log analysis", false},
		{"write to database", true},
		{"delete production resources", true},
		{"create new deployment", true},
		{"update configuration", true},
		{"read system logs", false},
	}
	for _, tt := range tests {
		got := containsWriteKeyword(tt.intent)
		if got != tt.want {
			t.Errorf("containsWriteKeyword(%q) = %v, want %v", tt.intent, got, tt.want)
		}
	}
}
