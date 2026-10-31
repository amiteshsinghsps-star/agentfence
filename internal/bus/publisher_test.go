package bus

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestNewPublisher(t *testing.T) {
	p := NewPublisher(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})), "test")
	if p == nil {
		t.Fatal("expected non-nil publisher")
	}
	if len(p.subscribers) != 0 {
		t.Errorf("expected 0 subscribers, got %d", len(p.subscribers))
	}
}

func TestSubscribeAndPublish(t *testing.T) {
	p := NewPublisher(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})), "test")
	ch := p.Subscribe("agentfence.test.sub")

	select {
	case <-ch:
		t.Fatal("expected no message before publish")
	default:
	}

	p.publish("agentfence.test.sub", map[string]string{"msg": "hello"})

	select {
	case msg := <-ch:
		if len(msg) == 0 {
			t.Error("expected non-empty message")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for message")
	}
}

func TestSubscriberSlowDropped(t *testing.T) {
	p := NewPublisher(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})), "test")
	// Create a subscriber with a small buffer
	p.subscribers["test.slow"] = []chan []byte{make(chan []byte, 1)}

	// Fill the buffer
	p.publish("test.slow", "msg1")
	p.publish("test.slow", "msg2") // should be dropped

	// Should not panic
	p.publish("test.slow", "msg3") // should be dropped
}

func TestPublishDecision(t *testing.T) {
	p := NewPublisher(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})), "test")
	ch := p.Subscribe("agentfence.decisions.sess-1")

	p.PublishDecision("sess-1", map[string]string{"call": "test"}, map[string]string{"outcome": "ALLOW"})

	select {
	case msg := <-ch:
		if len(msg) == 0 {
			t.Error("expected non-empty decision message")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for decision message")
	}
}

func TestPublishSuspicion(t *testing.T) {
	p := NewPublisher(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})), "test")
	ch := p.Subscribe("agentfence.suspicion.sess-1")

	p.PublishSuspicion("sess-1", 0.75, 0.25, []string{"scope_violation"})

	select {
	case <-ch:
		// success
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for suspicion message")
	}
}

func TestPublishFence(t *testing.T) {
	p := NewPublisher(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})), "test")
	ch := p.Subscribe("agentfence.fences.sess-1")

	p.PublishFence("sess-1", "QUARANTINE", "suspicion_threshold", "test", time.Now())

	select {
	case <-ch:
		// success
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for fence message")
	}
}

func TestPublishSpawn(t *testing.T) {
	p := NewPublisher(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})), "test")
	ch := p.Subscribe("agentfence.spawns.parent-1")

	p.PublishSpawn("parent-1", "child-1", "HALF_OPEN", true)

	select {
	case <-ch:
		// success
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for spawn message")
	}
}

func TestPublishBlastRadius(t *testing.T) {
	p := NewPublisher(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})), "test")
	ch := p.Subscribe("agentfence.blastradius.sess-1")

	p.PublishBlastRadius("sess-1", map[string]int{"affected": 14})

	select {
	case <-ch:
		// success
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for blast radius message")
	}
}

func TestNATSPublisher(t *testing.T) {
	n := NewNATSPublisher(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})), "nats://localhost:4222")
	if err := n.Publish(nil, "test", []byte("hello")); err != nil {
		t.Errorf("Publish failed: %v", err)
	}
	if err := n.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}
