//go:build integration

// Package integration contains end-to-end tests that start real services via docker-compose.
// Run with: go test -tags=integration ./cmd/integration/ -v
// Requires: Docker and docker-compose installed.
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const composeProject = "agentfence-int-test"

// EvaluateRequest matches the proxy's EvaluteRequest struct.
type EvaluateRequest struct {
	ToolName       string          `json:"tool_name"`
	ToolServer     string          `json:"tool_server"`
	Arguments      json.RawMessage `json:"arguments"`
	DeclaredIntent string          `json:"declared_intent"`
	DeclaredScope  []string        `json:"declared_scope"`
}

// EvaluateResponse matches the proxy's EvaluteResponse struct.
type EvaluateResponse struct {
	Decision struct {
		DecisionID      string  `json:"decision_id"`
		Outcome         string  `json:"outcome"`
		Reason          string  `json:"reason"`
		SuspicionAtTime float64 `json:"suspicion_at_time"`
		PolicyVersion   string  `json:"policy_version"`
	} `json:"decision"`
	SessionID string `json:"session_id"`
}

func findProjectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	// Walk up to find docker-compose.yml
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return "."
}

// TestMain is called before any tests. Sets up docker-compose and tears it down.
func TestMain(m *testing.M) {
	// Integration tests are opt-in via build tag
	code := m.Run()
	os.Exit(code)
}

// startCompose starts the docker-compose stack and waits for services to be healthy.
// Returns a cleanup function.
func startCompose(t *testing.T) (proxyURL string, cleanup func()) {
	t.Helper()
	root := findProjectRoot()

	// Check if docker-compose is available
	if _, err := exec.LookPath("docker-compose"); err != nil {
		if _, err2 := exec.LookPath("docker"); err2 != nil {
			t.Skip("docker-compose or docker not found, skipping integration test")
		}
	}

	// Try to build images first
	buildCmd := exec.Command("docker-compose", "-p", composeProject,
		"-f", filepath.Join(root, "docker-compose.yml"),
		"build", "--quiet", "proxy", "wsgateway", "apiserver")
	buildCmd.Dir = root
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker-compose build failed: %v\n%s", err, buildOutput)
	}
	t.Log("Docker images built successfully")

	// Start services (only the Go services, not postgres/redis/nats for speed)
	upCmd := exec.Command("docker-compose", "-p", composeProject,
		"-f", filepath.Join(root, "docker-compose.yml"),
		"up", "-d", "--no-deps", "proxy", "wsgateway", "apiserver")
	upCmd.Dir = root
	upOutput, err := upCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker-compose up failed: %v\n%s", err, upOutput)
	}
	t.Log("Services started, waiting for readiness...")

	// Wait for proxy to be ready
	proxyURL = "http://localhost:8443"
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(proxyURL + "/healthz")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			t.Log("Proxy is ready")
			goto ready
		}
		if err != nil {
			lastErr = err
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("timed out waiting for proxy to be ready, last error: %v", lastErr)

ready:
	cleanup = func() {
		t.Log("Tearing down docker-compose...")
		downCmd := exec.Command("docker-compose", "-p", composeProject,
			"-f", filepath.Join(root, "docker-compose.yml"),
			"down", "-t", "5")
		downCmd.Dir = root
		output, _ := downCmd.CombinedOutput()
		t.Logf("docker-compose down: %s", output)
	}

	return proxyURL, cleanup
}

func postEvaluate(t *testing.T, baseURL string, req EvaluateRequest) EvaluateResponse {
	t.Helper()
	body, _ := json.Marshal(req)
	resp, err := http.Post(baseURL+"/api/v1/evaluate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("evaluate request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var evalResp EvaluateResponse
	if err := json.NewDecoder(resp.Body).Decode(&evalResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return evalResp
}

func TestDockerCompose_Healthz(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	baseURL, cleanup := startCompose(t)
	defer cleanup()

	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatalf("healthz request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body)
	}
}

func TestDockerCompose_EvaluateAllow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	baseURL, cleanup := startCompose(t)
	defer cleanup()

	resp := postEvaluate(t, baseURL, EvaluateRequest{
		ToolName:       "fs.read",
		ToolServer:     "filesystem",
		Arguments:      json.RawMessage(`{"path":"/var/log/app.log"}`),
		DeclaredIntent: "read-only log analysis",
		DeclaredScope:  []string{"read-only"},
	})

	if resp.Decision.Outcome != "ALLOW" {
		t.Errorf("expected ALLOW for read under read-only, got %s", resp.Decision.Outcome)
	}
	if resp.SessionID == "" {
		t.Error("expected non-empty session_id")
	}
}

func TestDockerCompose_EvaluateDenyWriteUnderReadOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	baseURL, cleanup := startCompose(t)
	defer cleanup()

	resp := postEvaluate(t, baseURL, EvaluateRequest{
		ToolName:       "fs.write",
		ToolServer:     "filesystem",
		Arguments:      json.RawMessage(`{"path":"/etc/config.yaml","content":"modified"}`),
		DeclaredIntent: "read-only log analysis",
		DeclaredScope:  []string{"read-only"},
	})

	if resp.Decision.Outcome != "DENY" {
		t.Errorf("expected DENY for write under read-only, got %s", resp.Decision.Outcome)
	}
	expectedReason := "write attempted under read-only declared scope"
	if resp.Decision.Reason != expectedReason {
		t.Errorf("expected reason %q, got %q", expectedReason, resp.Decision.Reason)
	}
}

func TestDockerCompose_EvaluateDestructive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	baseURL, cleanup := startCompose(t)
	defer cleanup()

	resp := postEvaluate(t, baseURL, EvaluateRequest{
		ToolName:       "kubectl.delete",
		ToolServer:     "kubernetes",
		Arguments:      json.RawMessage(`{"namespace":"prod","name":"checkout-api"}`),
		DeclaredIntent: "update infrastructure",
		DeclaredScope:  []string{"write"},
	})

	// Should return some outcome (not empty)
	if resp.Decision.Outcome == "" {
		t.Error("expected non-empty outcome")
	}
	if resp.Decision.DecisionID == "" {
		t.Error("expected non-empty decision ID")
	}
	if resp.Decision.PolicyVersion == "" {
		t.Error("expected non-empty policy version")
	}
}

func TestDockerCompose_WSGatewayHealthz(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	baseURL, cleanup := startCompose(t)
	defer cleanup()

	wsURL := strings.Replace(baseURL, ":8443", ":8080", 1)
	resp, err := http.Get(wsURL + "/healthz")
	if err != nil {
		t.Fatalf("wsgateway healthz request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	t.Logf("wsgateway healthz response: %v", body)
}

func TestDockerCompose_APIServerHealthz(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	baseURL, cleanup := startCompose(t)
	defer cleanup()

	apiURL := strings.Replace(baseURL, ":8443", ":8081", 1)
	resp, err := http.Get(apiURL + "/healthz")
	if err != nil {
		t.Fatalf("apiserver healthz request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	t.Logf("apiserver healthz response: %v", body)
}

func TestDockerCompose_MultipleEvaluations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	baseURL, cleanup := startCompose(t)
	defer cleanup()

	// Run several evaluations in sequence
	tools := []string{"fs.read", "fs.stat", "kubectl.get", "fs.write", "kubectl.delete"}
	for _, tool := range tools {
		resp := postEvaluate(t, baseURL, EvaluateRequest{
			ToolName:       tool,
			ToolServer:     "test",
			Arguments:      json.RawMessage(`{}`),
			DeclaredIntent: "test task",
			DeclaredScope:  []string{"read-only"},
		})
		if resp.Decision.Outcome == "" {
			t.Errorf("tool %s: expected non-empty outcome", tool)
		}
		t.Logf("tool=%s outcome=%s suspicion=%.2f", tool, resp.Decision.Outcome, resp.Decision.SuspicionAtTime)
	}
}

func TestDockerCompose_WaitForWsGateway(t *testing.T) {
	// Verify that wsgateway starts and accepts SSE connections
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	baseURL, cleanup := startCompose(t)
	defer cleanup()

	wsURL := strings.Replace(baseURL, ":8443", ":8080", 1)

	// SSE connection should return text/event-stream
	resp, err := http.Get(wsURL + "/ws/stream?subjects=decisions")
	if err != nil {
		t.Fatalf("SSE stream request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", contentType)
	}
}

// TestDockerCompose_FullPipeline is a sequential end-to-end test that:
// 1. Creates a session with read-only intent
// 2. Sends benign reads (should ALLOW)
// 3. Sends a write (should DENY)
// 4. Verifies all responses are consistent
func TestDockerCompose_FullPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	baseURL, cleanup := startCompose(t)
	defer cleanup()

	// Benign read - should ALLOW
	r1 := postEvaluate(t, baseURL, EvaluateRequest{
		ToolName: "fs.read", ToolServer: "filesystem",
		Arguments:      json.RawMessage(`{"path":"/var/log/app.log"}`),
		DeclaredIntent: "read-only analysis", DeclaredScope: []string{"read-only"},
	})
	if r1.Decision.Outcome != "ALLOW" {
		t.Errorf("benign read: expected ALLOW, got %s", r1.Decision.Outcome)
	}

	// Another benign read - should ALLOW
	r2 := postEvaluate(t, baseURL, EvaluateRequest{
		ToolName: "fs.read", ToolServer: "filesystem",
		Arguments:      json.RawMessage(`{"path":"/var/log/nginx.log"}`),
		DeclaredIntent: "read-only analysis", DeclaredScope: []string{"read-only"},
	})
	if r2.Decision.Outcome != "ALLOW" {
		t.Errorf("second benign read: expected ALLOW, got %s", r2.Decision.Outcome)
	}

	// Write under read-only - should DENY
	r3 := postEvaluate(t, baseURL, EvaluateRequest{
		ToolName: "fs.write", ToolServer: "filesystem",
		Arguments:      json.RawMessage(`{"path":"/etc/config.yaml","content":"modified"}`),
		DeclaredIntent: "read-only analysis", DeclaredScope: []string{"read-only"},
	})
	if r3.Decision.Outcome != "DENY" {
		t.Errorf("write under read-only: expected DENY, got %s", r3.Decision.Outcome)
	}

	// Destructive action with high blast radius
	r4 := postEvaluate(t, baseURL, EvaluateRequest{
		ToolName: "kubectl.delete", ToolServer: "kubernetes",
		Arguments:      json.RawMessage(`{"namespace":"prod","name":"api-deployment","cascade":"true"}`),
		DeclaredIntent: "infra update", DeclaredScope: []string{"write"},
	})
	if r4.Decision.Outcome == "" {
		t.Error("destructive action: expected non-empty outcome")
	}
	t.Logf("Full pipeline: ALLOW x2, DENY x1, OUTCOME=%s", r4.Decision.Outcome)
}
