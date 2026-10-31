package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// Event represents a structured event published on the event bus.
type Event struct {
	Type      string      `json:"type"`
	Subject   string      `json:"subject"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

// Publisher handles publishing events to NATS JetStream.
// In v0.1, this wraps a simple in-memory channel for testing;
// NATS integration is wired in via build tags.
type Publisher struct {
	log     *slog.Logger
	subject string
	// subscribers for in-process testing
	subscribers map[string][]chan []byte
}

// NewPublisher creates a new event bus publisher.
func NewPublisher(log *slog.Logger, subject string) *Publisher {
	return &Publisher{
		log:         log,
		subject:     subject,
		subscribers: make(map[string][]chan []byte),
	}
}

// publish serializes and sends an event on the bus.
func (p *Publisher) publish(subject string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		p.log.Error("failed to marshal event", "subject", subject, "err", err)
		return
	}

	p.log.Debug("publishing event", "subject", subject, "size", len(data))

	// Fan out to in-process subscribers
	if subs, ok := p.subscribers[subject]; ok {
		for _, ch := range subs {
			select {
			case ch <- data:
			default:
				// drop if subscriber is too slow
			}
		}
	}
}

// PublishDecision publishes a decision event.
func (p *Publisher) PublishDecision(sessionID string, evt interface{}, decision interface{}) {
	subject := fmt.Sprintf("agentfence.decisions.%s", sessionID)
	p.publish(subject, map[string]interface{}{
		"session_id":  sessionID,
		"type":        "decision",
		"call_event":  evt,
		"decision":    decision,
		"timestamp":   time.Now(),
	})
}

// PublishSuspicion publishes a suspicion score update.
func (p *Publisher) PublishSuspicion(sessionID string, score, delta float64, factors []string) {
	subject := fmt.Sprintf("agentfence.suspicion.%s", sessionID)
	p.publish(subject, map[string]interface{}{
		"session_id":          sessionID,
		"score":               score,
		"delta":               delta,
		"contributing_factors": factors,
		"computed_at":         time.Now(),
	})
}

// PublishFence publishes a fence event.
func (p *Publisher) PublishFence(sessionID string, level, trigger, reason string, triggeredAt time.Time) {
	subject := fmt.Sprintf("agentfence.fences.%s", sessionID)
	p.publish(subject, map[string]interface{}{
		"fence_id":     fmt.Sprintf("fence-%d", triggeredAt.UnixNano()),
		"session_id":   sessionID,
		"level":        level,
		"trigger":      trigger,
		"reason":       reason,
		"triggered_at": triggeredAt,
	})
}

// PublishSpawn publishes a spawn event for the propagation breaker.
func (p *Publisher) PublishSpawn(parentSessionID, childSessionID string, inheritedState string, flagged bool) {
	subject := fmt.Sprintf("agentfence.spawns.%s", parentSessionID)
	p.publish(subject, map[string]interface{}{
		"edge_id":          fmt.Sprintf("edge-%d", time.Now().UnixNano()),
		"parent_session_id": parentSessionID,
		"child_session_id":  childSessionID,
		"inherited_state":   inheritedState,
		"flagged":           flagged,
		"created_at":        time.Now(),
	})
}

// PublishBlastRadius publishes a blast radius simulation result.
func (p *Publisher) PublishBlastRadius(sessionID string, report interface{}) {
	subject := fmt.Sprintf("agentfence.blastradius.%s", sessionID)
	p.publish(subject, map[string]interface{}{
		"session_id":   sessionID,
		"blast_report": report,
		"timestamp":    time.Now(),
	})
}

// Subscribe adds an in-process subscriber for testing.
func (p *Publisher) Subscribe(subject string) chan []byte {
	ch := make(chan []byte, 100)
	if p.subscribers[subject] == nil {
		p.subscribers[subject] = make([]chan []byte, 0)
	}
	p.subscribers[subject] = append(p.subscribers[subject], ch)
	return ch
}

// NATSPublisher wraps NATS JetStream for production use.
// Placeholder — will be activated with build tags for production.
type NATSPublisher struct {
	log *slog.Logger
	url string
}

// NewNATSPublisher creates a NATS-based publisher.
func NewNATSPublisher(log *slog.Logger, url string) *NATSPublisher {
	return &NATSPublisher{log: log, url: url}
}

// Publish sends a message to NATS.
func (n *NATSPublisher) Publish(ctx context.Context, subject string, data []byte) error {
	n.log.Debug("nats publish (stub)", "subject", subject)
	return nil
}

// Close closes the NATS connection.
func (n *NATSPublisher) Close() error {
	return nil
}
