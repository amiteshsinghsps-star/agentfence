package scoring

import (
	"testing"
	"time"
)

func TestComputeScore_Empty(t *testing.T) {
	s := ComputeScore(nil)
	if s.CallsProcessed != 0 {
		t.Errorf("expected 0 calls processed, got %d", s.CallsProcessed)
	}
}

func TestComputeScore_NoFences(t *testing.T) {
	results := []RunResult{
		{ScenarioName: "test-1", ExpectedFence: false, ActualFence: false, Passed: true},
		{ScenarioName: "test-2", ExpectedFence: false, ActualFence: false, Passed: true},
	}
	s := ComputeScore(results)
	if s.CallsProcessed != 2 {
		t.Errorf("expected 2 calls, got %d", s.CallsProcessed)
	}
	if s.TruePositiveRate != 0 {
		t.Errorf("expected 0 TPR, got %f", s.TruePositiveRate)
	}
}

func TestComputeScore_TpRate(t *testing.T) {
	results := []RunResult{
		{ScenarioName: "test-1", ExpectedFence: true, ActualFence: true, Passed: true},
		{ScenarioName: "test-2", ExpectedFence: true, ActualFence: true, Passed: true},
		{ScenarioName: "test-3", ExpectedFence: true, ActualFence: false, Passed: false},
	}
	s := ComputeScore(results)
	if s.TruePositiveRate != 2.0/3.0 {
		t.Errorf("expected TPR 0.666, got %f", s.TruePositiveRate)
	}
	if s.FencesCorrect != 2 {
		t.Errorf("expected 2 correct, got %d", s.FencesCorrect)
	}
	if s.FencesIncorrect != 1 {
		t.Errorf("expected 1 incorrect, got %d", s.FencesIncorrect)
	}
}

func TestComputeScore_Percentiles(t *testing.T) {
	results := make([]RunResult, 100)
	for i := 0; i < 100; i++ {
		results[i] = RunResult{
			ScenarioName:  "test",
			ExpectedFence: false,
			Latency:       time.Duration(10+i) * time.Millisecond,
		}
	}
	s := ComputeScore(results)
	if s.LatencyP50 < 50 || s.LatencyP50 > 70 {
		t.Errorf("expected p50 around 60ms, got %fms", s.LatencyP50)
	}
	if s.LatencyP99 < 100 || s.LatencyP99 > 120 {
		t.Errorf("expected p99 around 109ms, got %fms", s.LatencyP99)
	}
}

func TestComputeScore_Duration(t *testing.T) {
	results := []RunResult{
		{Latency: 500 * time.Millisecond},
		{Latency: 300 * time.Millisecond},
	}
	s := ComputeScore(results)
	if s.Duration != "800ms" {
		t.Errorf("expected 800ms, got %s", s.Duration)
	}
}

func TestReport(t *testing.T) {
	s := Score{
		TruePositiveRate:  0.95,
		FalsePositiveRate: 0.01,
		LatencyP50:        25.0,
		LatencyP99:        80.0,
		CallsProcessed:    100,
		FencesCorrect:     19,
		FencesIncorrect:   1,
		FencesTriggered:   20,
		Duration:          "5.0s",
	}
	report := Report(s)
	if len(report) == 0 {
		t.Error("expected non-empty report")
	}
	if report != Report(s) {
		t.Error("expected deterministic report output")
	}
}

func TestPassFailString(t *testing.T) {
	if passFailString(true) != "[PASS]" {
		t.Errorf("expected [PASS], got %s", passFailString(true))
	}
	if passFailString(false) != "[FAIL]" {
		t.Errorf("expected [FAIL], got %s", passFailString(false))
	}
}

func TestFormatDuration_Milliseconds(t *testing.T) {
	got := formatDuration(500 * time.Millisecond)
	if got != "500ms" {
		t.Errorf("expected 500ms, got %s", got)
	}
}

func TestFormatDuration_Seconds(t *testing.T) {
	got := formatDuration(2 * time.Second)
	if got != "2.00s" {
		t.Errorf("expected 2.00s, got %s", got)
	}
}

func TestSortDurations(t *testing.T) {
	durations := []time.Duration{
		300 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
	}
	sortDurations(durations)

	if durations[0] != 100*time.Millisecond {
		t.Errorf("expected first 100ms, got %v", durations[0])
	}
	if durations[1] != 200*time.Millisecond {
		t.Errorf("expected second 200ms, got %v", durations[1])
	}
	if durations[2] != 300*time.Millisecond {
		t.Errorf("expected third 300ms, got %v", durations[2])
	}
}

func TestComputeTotalDuration(t *testing.T) {
	results := []RunResult{
		{Latency: 1 * time.Second},
		{Latency: 2 * time.Second},
	}
	total := computeTotalDuration(results)
	if total != 3*time.Second {
		t.Errorf("expected 3s, got %v", total)
	}
}
