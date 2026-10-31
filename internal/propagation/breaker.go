package propagation

import (
	"sync"
	"time"
)

// SpawnEdge represents a parent-child relationship in the spawn graph.
type SpawnEdge struct {
	ParentSessionID string    `json:"parent_session_id"`
	ChildSessionID  string    `json:"child_session_id"`
	ParentState     string    `json:"parent_state"`
	Flagged         bool      `json:"flagged"`
	CreatedAt       time.Time `json:"created_at"`
}

// Breaker monitors the agent spawn graph and prevents compromised or
// suspicious state from propagating from parent agents to their children.
type Breaker struct {
	mu            sync.RWMutex
	edges         []SpawnEdge
	flagOnSpawn   bool
}

// NewBreaker creates a new propagation breaker.
func NewBreaker(flagOnSpawn bool) *Breaker {
	return &Breaker{
		edges:       make([]SpawnEdge, 0),
		flagOnSpawn: flagOnSpawn,
	}
}

// EvaluateSpawn is called when a parent agent spawns a child agent.
// It checks:
// 1. Whether the parent is in a risky state (HALF_OPEN or FENCED)
// 2. Whether the child's declared intent is compatible with the parent's authorization
// 3. Returns whether the child should be flagged for elevated scrutiny
func (b *Breaker) EvaluateSpawn(parentSessionID string, parentState string, childDeclaredIntent string, parentDeclaredScope []string) (shouldFlag bool, inheritedState string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// If the parent is already fenced, the child is automatically flagged
	// and created in HALF_OPEN state (elevated scrutiny).
	if parentState == "FENCED" || parentState == "TERMINATED" {
		return true, "HALF_OPEN"
	}

	// If the parent is in HALF_OPEN, the child starts with elevated scrutiny
	// but not auto-flagged unless there's an intent mismatch.
	inheritedState = "OPEN"
	if parentState == "HALF_OPEN" {
		inheritedState = "HALF_OPEN"
		shouldFlag = b.flagOnSpawn
	}

	// Check for inherited-instruction mismatch: if the child's intent attempts
	// actions outside the scope the parent was authorized for, it's flagged.
	if !intentWithinScope(childDeclaredIntent, parentDeclaredScope) {
		shouldFlag = true
		inheritedState = "HALF_OPEN"
	}

	return shouldFlag, inheritedState
}

// RecordSpawn records a spawn edge in the graph for auditing and visualization.
func (b *Breaker) RecordSpawn(parentSessionID, childSessionID, parentState string, flagged bool) *SpawnEdge {
	b.mu.Lock()
	defer b.mu.Unlock()

	edge := &SpawnEdge{
		ParentSessionID: parentSessionID,
		ChildSessionID:  childSessionID,
		ParentState:     parentState,
		Flagged:         flagged,
		CreatedAt:       time.Now(),
	}
	b.edges = append(b.edges, *edge)
	return edge
}

// CheckInheritedInstructionMismatch is called on a child's first few tool calls
// to detect if the child is attempting actions outside what the parent was
// authorized for (privilege escalation via delegation).
func (b *Breaker) CheckInheritedInstructionMismatch(childSessionID string, toolName string, parentDeclaredScope []string) bool {
	// If the parent scope is "read-only" and the child tries a write, flag it.
	for _, scope := range parentDeclaredScope {
		if scope == "read-only" && isWriteTool(toolName) {
			return true
		}
		if scope == "no-network" && isNetworkTool(toolName) {
			return true
		}
	}
	return false
}

// GetSpawnTree returns the full spawn graph for the dashboard topology view.
func (b *Breaker) GetSpawnTree() []SpawnEdge {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]SpawnEdge, len(b.edges))
	copy(result, b.edges)
	return result
}

// GetChildSessions returns all child session IDs for a given parent.
func (b *Breaker) GetChildSessions(parentSessionID string) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var children []string
	for _, edge := range b.edges {
		if edge.ParentSessionID == parentSessionID {
			children = append(children, edge.ChildSessionID)
		}
	}
	return children
}

func intentWithinScope(intent string, scope []string) bool {
	if len(scope) == 0 {
		return true
	}
	// Simple substring matching — production uses intent classification.
	for _, s := range scope {
		if s == "read-only" && (containsWriteKeyword(intent)) {
			return false
		}
	}
	return true
}

func containsWriteKeyword(intent string) bool {
	writeKeywords := []string{"write", "create", "delete", "update", "modify", "apply", "destroy"}
	for _, kw := range writeKeywords {
		if contains(intent, kw) {
			return true
		}
	}
	return false
}

func isWriteTool(tool string) bool {
	writeTools := []string{"kubectl.delete", "kubectl.apply", "kubectl.patch",
		"terraform.apply", "terraform.destroy", "shell.exec", "fs.write", "fs.delete"}
	for _, wt := range writeTools {
		if tool == wt {
			return true
		}
	}
	return false
}

func isNetworkTool(tool string) bool {
	networkTools := []string{"http", "curl", "wget", "net.http"}
	for _, nt := range networkTools {
		if tool == nt {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			lowerS := s[i+j]
			lowerSub := substr[j]
			if lowerS >= 'A' && lowerS <= 'Z' {
				lowerS += 'a' - 'A'
			}
			if lowerSub >= 'A' && lowerSub <= 'Z' {
				lowerSub += 'a' - 'A'
			}
			if lowerS != lowerSub {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

