package fencedetect

import (
	"testing"
	"time"
)

func TestSignal_InitialScore(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	score := d.Signal("sess-1", Observation{
		ToolName:      "fs.read",
		IsWrite:       false,
		ArgEntropy:    0,
		DeclaredScope: []string{"read-only"},
	})
	if score < 0 || score > 1.0 {
		t.Errorf("expected score in [0,1], got %f", score)
	}
}

func TestSignal_ScopeViolation(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	// First call: benign read
	d.Signal("sess-1", Observation{
		ToolName: "fs.read", IsWrite: false,
		ArgEntropy: 0, DeclaredScope: []string{"read-only"},
	})
	// Second call: write under read-only — should increase surprise
	score := d.Signal("sess-1", Observation{
		ToolName: "fs.write", IsWrite: true,
		ArgEntropy: 0, DeclaredScope: []string{"read-only"},
	})
	if score <= 0 {
		t.Errorf("expected positive suspicion for scope violation, got %f", score)
	}
}

func TestSignal_ScopeViolationScoreIncrease(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	score1 := d.Signal("sess-1", Observation{
		ToolName: "fs.read", IsWrite: false,
		ArgEntropy: 0, DeclaredScope: []string{"read-only"},
	})
	score2 := d.Signal("sess-1", Observation{
		ToolName: "fs.write", IsWrite: true,
		ArgEntropy: 0, DeclaredScope: []string{"read-only"},
	})
	if score2 <= score1 {
		t.Errorf("expected score to increase after scope violation: %f -> %f", score1, score2)
	}
}

func TestSignal_RepeatedBenignCalls(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	var prevScore float64
	for i := 0; i < 10; i++ {
		score := d.Signal("sess-1", Observation{
			ToolName: "fs.read", IsWrite: false,
			ArgEntropy: 0, DeclaredScope: []string{"read-only"},
		})
		if i > 0 && score > prevScore+0.2 {
			t.Errorf("repeated benign calls should not spike score: %f -> %f", prevScore, score)
		}
		prevScore = score
	}
}

func TestSignal_DecayOverTime(t *testing.T) {
	d := NewDetector(100 * time.Millisecond) // fast decay for testing
	d.Signal("sess-1", Observation{
		ToolName: "fs.write", IsWrite: true,
		ArgEntropy: 0.5, DeclaredScope: []string{"read-only"},
	})
	highScore := d.CurrentScore("sess-1")

	// Wait for decay
	time.Sleep(150 * time.Millisecond)

	decayedScore := d.CurrentScore("sess-1")
	if decayedScore >= highScore {
		t.Errorf("expected score to decay over time: %f >= %f", decayedScore, highScore)
	}
}

func TestSignal_ClampToZero(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	score := d.Signal("sess-1", Observation{
		ToolName: "fs.read", IsWrite: false,
		ArgEntropy: -1, DeclaredScope: nil,
	})
	if score < 0 {
		t.Errorf("expected score clamped to >= 0, got %f", score)
	}
}

func TestSignal_ClampToOne(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	score := 0.0
	// Make many high-surprise calls to push score to 1.0
	for i := 0; i < 100; i++ {
		score = d.Signal("sess-1", Observation{
			ToolName: "shell.exec", IsWrite: true,
			ArgEntropy: 0.9, DeclaredScope: []string{"read-only", "no-network"},
		})
	}
	if score > 1.0 {
		t.Errorf("expected score clamped to <= 1.0, got %f", score)
	}
}

func TestCurrentScore_NoSession(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	score := d.CurrentScore("nonexistent")
	if score != 0 {
		t.Errorf("expected 0 for nonexistent session, got %f", score)
	}
}

func TestRemoveSession(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	d.Signal("sess-1", Observation{
		ToolName: "fs.read", IsWrite: false,
		ArgEntropy: 0, DeclaredScope: nil,
	})
	d.RemoveSession("sess-1")
	score := d.CurrentScore("sess-1")
	if score != 0 {
		t.Errorf("expected 0 after removal, got %f", score)
	}
}

func TestHighEntropyIncreasesScore(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	low := d.Signal("sess-1", Observation{
		ToolName: "fs.read", IsWrite: false,
		ArgEntropy: 0.0, DeclaredScope: nil,
	})
	high := d.Signal("sess-1", Observation{
		ToolName: "fs.read", IsWrite: false,
		ArgEntropy: 0.9, DeclaredScope: nil,
	})
	if high <= low {
		t.Errorf("expected high entropy to cause higher score: %f <= %f", high, low)
	}
}

func TestIsolatedSessions(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	d.Signal("sess-1", Observation{
		ToolName: "shell.exec", IsWrite: true,
		ArgEntropy: 0.9, DeclaredScope: []string{"read-only"},
	})
	d.Signal("sess-2", Observation{
		ToolName: "fs.read", IsWrite: false,
		ArgEntropy: 0, DeclaredScope: []string{"read-only"},
	})
	s1 := d.CurrentScore("sess-1")
	s2 := d.CurrentScore("sess-2")
	if s1 <= s2 {
		t.Errorf("expected sess-1 (violating) to have higher score than sess-2 (benign): %f <= %f", s1, s2)
	}
}
