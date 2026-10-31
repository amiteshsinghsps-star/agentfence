package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Record represents a single decision record in the audit chain.
type Record struct {
	CallID          string  `json:"call_id"`
	SessionID       string  `json:"session_id"`
	ToolName        string  `json:"tool_name"`
	Outcome         string  `json:"outcome"`
	Reason          string  `json:"reason"`
	SuspicionAtTime float64 `json:"suspicion_at_time"`
	DecidedAt       string  `json:"decided_at"`
	PrevHash        string  `json:"prev_hash"`
	ThisHash        string  `json:"this_hash"`
}

// bodyFields contains the fields that make up the record body (excludes chain metadata).
type bodyFields struct {
	CallID          string  `json:"call_id"`
	SessionID       string  `json:"session_id"`
	ToolName        string  `json:"tool_name"`
	Outcome         string  `json:"outcome"`
	Reason          string  `json:"reason"`
	SuspicionAtTime float64 `json:"suspicion_at_time"`
	DecidedAt       string  `json:"decided_at"`
}

// Chain provides a tamper-evident hash-chained audit trail.
type Chain struct {
	mu       sync.Mutex
	records  []Record
	lastHash string
}

// NewChain creates a new in-memory audit chain.
func NewChain() *Chain {
	return &Chain{
		records:  make([]Record, 0),
		lastHash: "GENESIS",
	}
}

// Append writes a new decision record, chaining it to the previous record's hash.
func (c *Chain) Append(ctx context.Context, callID, sessionID, toolName, outcome, reason string, suspicionAtTime float64) (*Record, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	rec := Record{
		CallID:          callID,
		SessionID:       sessionID,
		ToolName:        toolName,
		Outcome:         outcome,
		Reason:          reason,
		SuspicionAtTime: suspicionAtTime,
		DecidedAt:       time.Now().UTC().Format(time.RFC3339Nano),
		PrevHash:        c.lastHash,
	}

	// Compute this_hash = SHA256(prev_hash || canonical_body)
	// We use only the body fields (excluding chain metadata) for deterministic hashing.
	body := bodyFields{
		CallID: rec.CallID, SessionID: rec.SessionID, ToolName: rec.ToolName,
		Outcome: rec.Outcome, Reason: rec.Reason,
		SuspicionAtTime: rec.SuspicionAtTime, DecidedAt: rec.DecidedAt,
	}
	canonical, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal record body: %w", err)
	}

	h := sha256.Sum256(append([]byte(rec.PrevHash), canonical...))
	rec.ThisHash = hex.EncodeToString(h[:])

	c.records = append(c.records, rec)
	c.lastHash = rec.ThisHash

	return &rec, nil
}

// VerifyChain walks all records and confirms hash-chain integrity.
func (c *Chain) VerifyChain(ctx context.Context) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, rec := range c.records {
		expectedPrev := "GENESIS"
		if i > 0 {
			expectedPrev = c.records[i-1].ThisHash
		}

		if rec.PrevHash != expectedPrev {
			return false, fmt.Errorf("chain broken at index %d (call_id=%s): prev_hash mismatch", i, rec.CallID)
		}

		body := bodyFields{
			CallID: rec.CallID, SessionID: rec.SessionID, ToolName: rec.ToolName,
			Outcome: rec.Outcome, Reason: rec.Reason,
			SuspicionAtTime: rec.SuspicionAtTime, DecidedAt: rec.DecidedAt,
		}
		canonical, err := json.Marshal(body)
		if err != nil {
			return false, fmt.Errorf("failed to marshal body at index %d: %w", i, err)
		}
		h := sha256.Sum256(append([]byte(rec.PrevHash), canonical...))
		expectedHash := hex.EncodeToString(h[:])

		if rec.ThisHash != expectedHash {
			return false, fmt.Errorf("chain broken at index %d (call_id=%s): hash mismatch", i, rec.CallID)
		}
	}

	return true, nil
}

// VerifyRange walks a range of records and confirms hash-chain integrity.
func (c *Chain) VerifyRange(ctx context.Context, fromIdx, toIdx int) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if fromIdx < 0 || toIdx >= len(c.records) || fromIdx > toIdx {
		return false, fmt.Errorf("invalid range [%d, %d] for %d records", fromIdx, toIdx, len(c.records))
	}

	for i := fromIdx; i <= toIdx; i++ {
		rec := c.records[i]
		expectedPrev := "GENESIS"
		if i > 0 {
			expectedPrev = c.records[i-1].ThisHash
		}

		if rec.PrevHash != expectedPrev {
			return false, fmt.Errorf("chain broken at index %d (call_id=%s)", i, rec.CallID)
		}

		body := bodyFields{
			CallID: rec.CallID, SessionID: rec.SessionID, ToolName: rec.ToolName,
			Outcome: rec.Outcome, Reason: rec.Reason,
			SuspicionAtTime: rec.SuspicionAtTime, DecidedAt: rec.DecidedAt,
		}
		canonical, err := json.Marshal(body)
		if err != nil {
			return false, fmt.Errorf("failed to marshal body at index %d: %w", i, err)
		}
		h := sha256.Sum256(append([]byte(rec.PrevHash), canonical...))
		expectedHash := hex.EncodeToString(h[:])

		if rec.ThisHash != expectedHash {
			return false, fmt.Errorf("chain broken at index %d (call_id=%s)", i, rec.CallID)
		}
	}

	return true, nil
}

// GetRecords returns all audit records.
func (c *Chain) GetRecords() []Record {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make([]Record, len(c.records))
	copy(result, c.records)
	return result
}

// GetRecordCount returns the total number of audit records.
func (c *Chain) GetRecordCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.records)
}

// ExportNDJSON serializes records as newline-delimited JSON for cold storage export.
func (c *Chain) ExportNDJSON(ctx context.Context, startIdx, endIdx int) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx >= len(c.records) {
		endIdx = len(c.records) - 1
	}
	if startIdx > endIdx {
		return nil, fmt.Errorf("invalid export range")
	}

	var result []byte
	for i := startIdx; i <= endIdx; i++ {
		data, err := json.Marshal(c.records[i])
		if err != nil {
			return nil, fmt.Errorf("failed to marshal record at index %d: %w", i, err)
		}
		result = append(result, data...)
		result = append(result, '\n')
	}

	return result, nil
}
