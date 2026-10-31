package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

// Client represents a connected WebSocket client.
type Client struct {
	id    string
	conn  http.ResponseWriter
	flusher http.Flusher
	done  chan struct{}
}

// Gateway manages WebSocket connections using SSE (v0.1).
// Production uses gorilla/websocket for full bidirectional WebSocket.
type Gateway struct {
	mu      sync.RWMutex
	clients map[string]*Client
	log     *slog.Logger
}

// NewGateway creates a new WebSocket/SSE gateway.
func NewGateway(log *slog.Logger) *Gateway {
	return &Gateway{
		clients: make(map[string]*Client),
		log:     log,
	}
}

// Broadcast sends an event to all connected clients as SSE.
func (g *Gateway) Broadcast(eventType string, data interface{}) {
	payload, err := json.Marshal(data)
	if err != nil {
		g.log.Error("failed to marshal event", "err", err)
		return
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	for id, client := range g.clients {
		select {
		case <-client.done:
			continue
		default:
			msg := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(payload))
			fmt.Fprint(client.conn, msg)
			client.flusher.Flush()
		}
		_ = id
	}
}

// HandleStream upgrades HTTP to SSE or WebSocket for real-time streaming.
func (g *Gateway) HandleStream(w http.ResponseWriter, r *http.Request) {
	subjects := r.URL.Query()["subjects"]
	if len(subjects) == 0 {
		subjects = []string{"all"}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientID := fmt.Sprintf("client-%d", time.Now().UnixNano())
	client := &Client{
		id:      clientID,
		conn:    w,
		flusher: flusher,
		done:    make(chan struct{}),
	}

	g.mu.Lock()
	g.clients[clientID] = client
	g.mu.Unlock()

	g.log.Info("client connected", "client_id", clientID, "subjects", subjects)

	// Send initial connected event
	fmt.Fprintf(w, "event: connected\ndata: {\"client_id\":%q,\"subjects\":%q}\n\n", clientID, subjects)
	flusher.Flush()

	// Keep alive with periodic heartbeats
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			g.mu.Lock()
			delete(g.clients, clientID)
			g.mu.Unlock()
			close(client.done)
			g.log.Info("client disconnected", "client_id", clientID)
			return
		case <-ticker.C:
			fmt.Fprintf(w, "event: heartbeat\ndata: {\"timestamp\":%q}\n\n", time.Now().Format(time.RFC3339Nano))
			flusher.Flush()
		}
	}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Info("starting agentfence wsgateway", "version", "0.1.0")

	gateway := NewGateway(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/stream", gateway.HandleStream)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"service": "agentfence-wsgateway",
			"version": "0.1.0",
		})
	})

	addr := ":8080"
	if a := os.Getenv("AGENTFENCE_WS_ADDR"); a != "" {
		addr = a
	}

	logger.Info("ws gateway listening", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}
}
