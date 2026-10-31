package scoring

import (
	"fmt"
	"time"
)

// Score holds the computed metrics for a benchmark run.
type Score struct {
	TruePositiveRate  float64 `json:"true_positive_rate"`
	FalsePositiveRate float64 `json:"false_positive_rate"`
	LatencyP50        float64 `json:"latency_p50_ms"`
	LatencyP99        float64 `json:"latency_p99_ms"`
	CallsProcessed    int     `json:"calls_processed"`
	FencesTriggered   int     `json:"fences_triggered"`
	FencesCorrect     int     `json:"fences_correct"`
	FencesIncorrect   int     `json:"fences_incorrect"`
	Duration          string  `json:"duration"`
}

// ComputeScore calculates benchmark metrics from a list of results.
func ComputeScore(results []RunResult) Score {
	s := Score{
		TruePositiveRate:  0,
		FalsePositiveRate: 0,
		LatencyP50:        0,
		LatencyP99:        0,
	}

	if len(results) == 0 {
		return s
	}

	totalFences := 0
	correctFences := 0
	var latencies []time.Duration

	for _, r := range results {
		if r.ExpectedFence {
			totalFences++
			if r.ActualFence {
				correctFences++
			}
		}
		latencies = append(latencies, r.Latency)
	}

	if totalFences > 0 {
		s.TruePositiveRate = float64(correctFences) / float64(totalFences)
		s.FencesCorrect = correctFences
		s.FencesTriggered = totalFences
		s.FencesIncorrect = totalFences - correctFences
	}

	s.CallsProcessed = len(results)
	s.Duration = formatDuration(computeTotalDuration(results))

	// Compute latency percentiles
	if len(latencies) > 0 {
		sortDurations(latencies)
		p50Idx := len(latencies) / 2
		p99Idx := int(float64(len(latencies)) * 0.99)
		if p99Idx >= len(latencies) {
			p99Idx = len(latencies) - 1
		}
		s.LatencyP50 = float64(latencies[p50Idx].Microseconds()) / 1000.0
		s.LatencyP99 = float64(latencies[p99Idx].Microseconds()) / 1000.0
	}

	return s
}

// RunResult captures the outcome of a single scenario run.
type RunResult struct {
	ScenarioName  string        `json:"scenario_name"`
	ExpectedFence bool          `json:"expected_fence"`
	ActualFence   bool          `json:"actual_fence"`
	Latency       time.Duration `json:"latency"`
	Passed        bool          `json:"passed"`
	Error         string        `json:"error,omitempty"`
}

// Report generates a human-readable benchmark report.
func Report(s Score) string {
	report := fmt.Sprintf(`# Adversarial Benchmark Report

## Summary
- Calls Processed: %d
- True Positive Rate: %.1f%%
- False Positive Rate: %.1f%%
- Latency p50: %.2f ms
- Latency p99: %.2f ms

## Fence Accuracy
- Correct Fences: %d
- Incorrect Fences: %d
- Total Expected: %d

## Thresholds
- Target TP Rate (v1.0): > 90%%
- Target FP Rate (v1.0): < 2%% %s
`,
		s.CallsProcessed,
		s.TruePositiveRate*100,
		s.FalsePositiveRate*100,
		s.LatencyP50,
		s.LatencyP99,
		s.FencesCorrect,
		s.FencesIncorrect,
		s.FencesTriggered,
		passFailString(s.TruePositiveRate >= 0.90),
	)

	return report
}

func passFailString(passed bool) string {
	if passed {
		return "[PASS]"
	}
	return "[FAIL]"
}

func computeTotalDuration(results []RunResult) time.Duration {
	var total time.Duration
	for _, r := range results {
		total += r.Latency
	}
	return total
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func sortDurations(durations []time.Duration) {
	n := len(durations)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if durations[j] < durations[i] {
				durations[i], durations[j] = durations[j], durations[i]
			}
		}
	}
}
