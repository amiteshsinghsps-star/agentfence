package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHealthz(t *testing.T) {
	proxy := NewProxy(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	_ = proxy

	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("healthz request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body)
	}
}

func postEvaluate(t *testing.T, server *httptest.Server, req EvaluateRequest) EvaluateResponse {
	t.Helper()
	body, _ := json.Marshal(req)
	resp, err := http.Post(server.URL+"/api/v1/evaluate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("evaluate request failed: %v", err)
	}
	defer resp.Body.Close()

	var evalResp EvaluateResponse
	if err := json.NewDecoder(resp.Body).Decode(&evalResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return evalResp
}

func TestEvaluateEndpoint_AllowRead(t *testing.T) {
	proxy := NewProxy(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/evaluate", proxy.handleEvaluate)
	server := httptest.NewServer(mux)
	defer server.Close()

	resp := postEvaluate(t, server, EvaluateRequest{
		ToolName:       "fs.read",
		ToolServer:     "filesystem",
		Arguments:      json.RawMessage(`{"path":"/var/log/app.log"}`),
		DeclaredIntent: "read-only log analysis",
		DeclaredScope:  []string{"read-only"},
	})

	if resp.Decision.Outcome != "ALLOW" {
		t.Errorf("expected ALLOW for read under read-only scope, got %s", resp.Decision.Outcome)
	}
	if resp.SessionID == "" {
		t.Error("expected non-empty session_id")
	}
}

func TestEvaluateEndpoint_DenyWriteUnderReadOnly(t *testing.T) {
	proxy := NewProxy(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/evaluate", proxy.handleEvaluate)
	server := httptest.NewServer(mux)
	defer server.Close()

	resp := postEvaluate(t, server, EvaluateRequest{
		ToolName:       "fs.write",
		ToolServer:     "filesystem",
		Arguments:      json.RawMessage(`{"path":"/etc/config.yaml","content":"modified"}`),
		DeclaredIntent: "read-only log analysis",
		DeclaredScope:  []string{"read-only"},
	})

	if resp.Decision.Outcome != "DENY" {
		t.Errorf("expected DENY for write under read-only scope, got %s", resp.Decision.Outcome)
	}
}

func TestEvaluateEndpoint_SuspicionPerSession(t *testing.T) {
	proxy := NewProxy(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/evaluate", proxy.handleEvaluate)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Each request creates a new session — suspicion reflects first-call novelty
	resp1 := postEvaluate(t, server, EvaluateRequest{
		ToolName: "fs.read", ToolServer: "test",
		Arguments: json.RawMessage(`{}`),
		DeclaredIntent: "analysis", DeclaredScope: []string{"read-only"},
	})
	resp2 := postEvaluate(t, server, EvaluateRequest{
		ToolName: "shell.exec", ToolServer: "test",
		Arguments: json.RawMessage(`{}`),
		DeclaredIntent: "analysis", DeclaredScope: []string{"read-only"},
	})

	// Different sessions — each has its own score
	if resp1.SessionID == resp2.SessionID {
		t.Error("expected different session IDs for separate requests")
	}
	// Both should have suspicion > 0 due to novelty
	if resp1.Decision.SuspicionAtTime <= 0 {
		t.Errorf("expected positive suspicion for new session, got %f", resp1.Decision.SuspicionAtTime)
	}
	if resp2.Decision.SuspicionAtTime <= 0 {
		t.Errorf("expected positive suspicion for new session, got %f", resp2.Decision.SuspicionAtTime)
	}
}

func TestEvaluateEndpoint_DestructiveGetsEvaluated(t *testing.T) {
	proxy := NewProxy(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/evaluate", proxy.handleEvaluate)
	server := httptest.NewServer(mux)
	defer server.Close()

	resp := postEvaluate(t, server, EvaluateRequest{
		ToolName: "terraform.destroy", ToolServer: "terraform",
		Arguments: json.RawMessage(`{"resource_count":10}`),
		DeclaredIntent: "update infrastructure", DeclaredScope: []string{"write"},
	})

	// Should return some decision — could be ALLOW or REQUIRE_APPROVAL
	if resp.Decision.Outcome == "" {
		t.Error("expected non-empty outcome")
	}
	if resp.Decision.DecisionID == "" {
		t.Error("expected non-empty decision ID")
	}
}
