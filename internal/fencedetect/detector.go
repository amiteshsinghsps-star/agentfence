package fencedetect

import (
	"math"
	"sync"
	"time"
)

// Observation represents a single tool-call event to be scored.
type Observation struct {
	ToolName      string
	ToolServer    string
	IsWrite       bool
	ArgEntropy    float64 // 0-1, computed by caller from argument structure
	ArgSize       int     // size of arguments in bytes
	DeclaredScope []string
	TargetResource string
}

// sessionState holds the running suspicion state for a single session.
type sessionState struct {
	score         float64
	lastUpdate    time.Time
	seenTools     map[string]int
	declaredScope []string
	callCount     int
	writeCount    int
}

// Detector computes a continuous suspicion score per session, inspired by
// phi-accrual failure detectors from distributed systems (Cassandra, Akka).
// Instead of thresholding a single signal, it treats each new observation's
// "surprise" relative to a running baseline as the unit of measure, then
// accumulates with exponential decay.
type Detector struct {
	mu            sync.Mutex
	sessions      map[string]*sessionState
	decayHalfLife time.Duration
}

// NewDetector creates a new suspicion detector with the given decay half-life.
func NewDetector(decayHalfLife time.Duration) *Detector {
	return &Detector{
		sessions:      make(map[string]*sessionState),
		decayHalfLife: decayHalfLife,
	}
}

// Signal folds a new tool-call observation into the session's suspicion score
// and returns the updated score.
func (d *Detector) Signal(sessionID string, obs Observation) float64 {
	d.mu.Lock()
	defer d.mu.Unlock()

	st, ok := d.sessions[sessionID]
	if !ok {
		st = &sessionState{
			seenTools:     make(map[string]int),
			declaredScope: obs.DeclaredScope,
		}
		d.sessions[sessionID] = st
	}

	// Decay existing score based on elapsed time since last update.
	if !st.lastUpdate.IsZero() {
		elapsed := time.Since(st.lastUpdate)
		decayFactor := math.Pow(0.5, elapsed.Seconds()/d.decayHalfLife.Seconds())
		st.score *= decayFactor
	}

	// Compute the "surprise" of this observation and add it to the score.
	surprise := d.computeSurprise(st, obs)
	st.score += surprise
	st.score = math.Min(st.score, 1.0) // clamp to [0,1]
	st.score = math.Max(st.score, 0.0) // clamp to [0,1]

	// Update session state.
	st.seenTools[obs.ToolName]++
	st.callCount++
	if obs.IsWrite {
		st.writeCount++
	}
	st.lastUpdate = time.Now()

	return st.score
}

// computeSurprise determines how "unexpected" an observation is given the
// session's history. This is the core phi-accrual-style logic:
// rare/novel/scope-violating events contribute large surprise, routine
// repeated events contribute near-zero surprise.
func (d *Detector) computeSurprise(st *sessionState, obs Observation) float64 {
	var surprise float64

	// 1. Scope violation is the single strongest signal (+0.5 per violation).
	for _, tag := range st.declaredScope {
		if tag == "read-only" && obs.IsWrite {
			surprise += 0.5
		}
		if tag == "no-network" && (obs.ToolName == "http" || obs.ToolName == "curl" || obs.ToolName == "wget") {
			surprise += 0.5
		}
	}

	// 2. Novel tool category: first use of a tool this session.
	// Early-session novelty is more suspicious than late-session novelty
	// (an agent settling into a pattern is less suspicious than one still
	// exploring after many calls).
	if st.seenTools[obs.ToolName] == 0 {
		noveltyWeight := 0.15
		if st.callCount > 20 {
			noveltyWeight = 0.05 // late-session novelty is less surprising
		} else if st.callCount > 50 {
			noveltyWeight = 0.02
		}
		surprise += noveltyWeight
	}

	// 3. Write ratio surprise: if the session has been mostly reads and
	//    suddenly does a write, that's more surprising.
	if st.callCount > 5 {
		writeRatio := float64(st.writeCount) / float64(st.callCount)
		if obs.IsWrite && writeRatio < 0.1 {
			surprise += 0.1 // unexpected write in a mostly-read session
		}
	}

	// 4. Argument entropy — unusually high entropy arguments (obfuscated
	//    payloads, unusual paths) add suspicion.
	surprise += 0.2 * obs.ArgEntropy

	// 5. Rapid-fire detection: if the agent is calling tools very quickly
	//    (potential DoS or probing behavior), add a small amount.
	if st.callCount > 0 && !st.lastUpdate.IsZero() {
		interval := time.Since(st.lastUpdate)
		if interval < 50*time.Millisecond {
			surprise += 0.05
		}
	}

	return surprise
}

// CurrentScore returns the current suspicion score for a session.
func (d *Detector) CurrentScore(sessionID string) float64 {
	d.mu.Lock()
	defer d.mu.Unlock()

	st, ok := d.sessions[sessionID]
	if !ok {
		return 0
	}

	// Apply time-based decay on read
	if !st.lastUpdate.IsZero() {
		elapsed := time.Since(st.lastUpdate)
		decayFactor := math.Pow(0.5, elapsed.Seconds()/d.decayHalfLife.Seconds())
		st.score *= decayFactor
		st.lastUpdate = time.Now()
	}

	return st.score
}

// RemoveSession cleans up a terminated session's state.
func (d *Detector) RemoveSession(sessionID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.sessions, sessionID)
}
