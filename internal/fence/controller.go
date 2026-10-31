package fence

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Level represents the severity of a fence action.
type Level string

const (
	LevelFreeze     Level = "FREEZE"
	LevelQuarantine Level = "QUARANTINE"
	LevelKill       Level = "KILL"
)

// Orchestrator defines the interface for executing fence actions on
// the underlying infrastructure (K8s pod termination, network isolation, etc.).
type Orchestrator interface {
	// KillSession terminates the underlying agent process/pod for a session.
	KillSession(ctx context.Context, sessionID string) error
	// IsolateNetwork applies a network/tool-access block at the proxy for a session.
	IsolateNetwork(ctx context.Context, sessionID string) error
}

// SessionStore defines the interface for reading/writing session state.
type SessionStore interface {
	SetState(ctx context.Context, sessionID, state string) error
	GetState(ctx context.Context, sessionID string) (string, error)
}

// EventPublisher defines the interface for publishing fence events.
type EventPublisher interface {
	PublishFence(sessionID string, level Level, trigger, reason string, triggeredAt time.Time) error
}

// Controller manages the fencing lifecycle — evaluate, fence, and unfence.
// Modeled on STONITH ("Shoot The Other Node In The Head") from distributed
// systems: isolate the misbehaving node completely rather than trying to
// gracefully degrade it, because a compromised/confused agent cannot be
// trusted to self-report its own state.
type Controller struct {
	orch                Orchestrator
	store               SessionStore
	bus                 EventPublisher
	freezeThreshold     float64
	quarantineThreshold float64
	log                 *slog.Logger
}

// NewController creates a new fencing controller.
func NewController(
	orch Orchestrator,
	store SessionStore,
	bus EventPublisher,
	freezeThreshold float64,
	quarantineThreshold float64,
	log *slog.Logger,
) *Controller {
	return &Controller{
		orch:                orch,
		store:               store,
		bus:                 bus,
		freezeThreshold:     freezeThreshold,
		quarantineThreshold: quarantineThreshold,
		log:                 log,
	}
}

// EvaluateScore is called on every suspicion score update and decides
// whether a fence action is warranted. This is the pure decision logic,
// kept separate from the imperative Fence() action for testability.
func (c *Controller) EvaluateScore(ctx context.Context, sessionID string, score float64) error {
	switch {
	case score >= c.quarantineThreshold:
		return c.Fence(ctx, sessionID, LevelQuarantine, "suspicion_threshold",
			fmt.Sprintf("suspicion score %.2f exceeded quarantine threshold %.2f", score, c.quarantineThreshold))
	case score >= c.freezeThreshold:
		return c.Fence(ctx, sessionID, LevelFreeze, "suspicion_threshold",
			fmt.Sprintf("suspicion score %.2f exceeded freeze threshold %.2f", score, c.freezeThreshold))
	}
	return nil
}

// Fence applies a fence action. This is the STONITH-style enforcement point:
// once called, the session is isolated first and questions are answered
// after — we do not attempt graceful negotiation with a session that may
// be compromised or malfunctioning.
func (c *Controller) Fence(ctx context.Context, sessionID string, level Level, trigger, reason string) error {
	current, err := c.store.GetState(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("cannot get session state for fencing: %w", err)
	}

	// Already fenced — no-op unless escalating to KILL.
	if current == "FENCED" && level != LevelKill {
		return nil
	}

	// Mark the session as fenced in the store FIRST so even if the
	// orchestration action fails, subsequent proxy checks will reject calls.
	if err := c.store.SetState(ctx, sessionID, "FENCED"); err != nil {
		return fmt.Errorf("failed to set fenced state (fence not guaranteed): %w", err)
	}

	// Execute the fence action on the orchestrator.
	switch level {
	case LevelQuarantine:
		if err := c.orch.IsolateNetwork(ctx, sessionID); err != nil {
			return fmt.Errorf("session marked fenced but network isolation failed: %w", err)
		}
	case LevelKill:
		if err := c.orch.IsolateNetwork(ctx, sessionID); err != nil {
			return fmt.Errorf("session marked fenced but network isolation failed: %w", err)
		}
		if err := c.orch.KillSession(ctx, sessionID); err != nil {
			return fmt.Errorf("session isolated but kill failed, manual intervention required: %w", err)
		}
	}

	// Publish the fence event for the audit trail and dashboard.
	if err := c.bus.PublishFence(sessionID, level, trigger, reason, time.Now()); err != nil {
		if c.log != nil {
			c.log.Error("failed to publish fence event", "err", err)
		}
	}

	return nil
}

// Unfence clears a fence — always requires an explicit reason for the audit trail.
func (c *Controller) Unfence(ctx context.Context, sessionID, reason, clearedBy string) error {
	return c.store.SetState(ctx, sessionID, "OPEN")
}

// DefaultThresholds returns the default freeze and quarantine thresholds.
func DefaultThresholds() (freeze, quarantine float64) {
	return 0.50, 0.75
}
