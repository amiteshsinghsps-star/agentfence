package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agentfence/agentfence/internal/audit"
	"github.com/agentfence/agentfence/internal/bus"
	"github.com/agentfence/agentfence/internal/fence"
	"github.com/agentfence/agentfence/internal/fencedetect"
	"github.com/agentfence/agentfence/internal/policy"
	"github.com/agentfence/agentfence/internal/propagation"
	"github.com/agentfence/agentfence/internal/session"
	"github.com/agentfence/agentfence/internal/simulate"
)

const agentID = "proxy-agent"

// ToolCallEvent represents an intercepted MCP tools/call.
type ToolCallEvent struct {
	CallID      string          `json:"call_id"`
	SessionID   string          `json:"session_id"`
	ToolName    string          `json:"tool_name"`
	ToolServer  string          `json:"tool_server"`
	Arguments   json.RawMessage `json:"arguments"`
	BlastClass  string          `json:"blast_class"`
	RequestedAt time.Time       `json:"requested_at"`
}

// Decision represents the outcome of the full evaluation pipeline.
type Decision struct {
	DecisionID      string                      `json:"decision_id"`
	Outcome         string                      `json:"outcome"`
	Reason          string                      `json:"reason"`
	BlastReport     *simulate.BlastRadiusReport `json:"blast_report,omitempty"`
	SuspicionAtTime float64                     `json:"suspicion_at_time"`
	PolicyVersion   string                      `json:"policy_version"`
}

// EvaluateRequest is the HTTP request body for the evaluate endpoint.
type EvaluateRequest struct {
	ToolName    string          `json:"tool_name"`
	ToolServer  string          `json:"tool_server"`
	Arguments   json.RawMessage `json:"arguments"`
	DeclaredIntent   string    `json:"declared_intent"`
	DeclaredScope    []string  `json:"declared_scope"`
}

// EvaluateResponse is the HTTP response for the evaluate endpoint.
type EvaluateResponse struct {
	Decision Decision `json:"decision"`
	SessionID string  `json:"session_id"`
}

// sessionStoreAdapter adapts session.Manager to fence.SessionStore.
type sessionStoreAdapter struct{ mgr *session.Manager }

func (a *sessionStoreAdapter) SetState(ctx context.Context, sessionID, state string) error {
	return a.mgr.SetStateString(ctx, sessionID, state)
}
func (a *sessionStoreAdapter) GetState(ctx context.Context, sessionID string) (string, error) {
	return a.mgr.GetState(ctx, sessionID)
}

// eventPublisherAdapter adapts bus.Publisher to fence.EventPublisher.
type eventPublisherAdapter struct{ pub *bus.Publisher }

func (a *eventPublisherAdapter) PublishFence(sessionID string, level fence.Level, trigger, reason string, triggeredAt time.Time) error {
	a.pub.PublishFence(sessionID, string(level), trigger, reason, triggeredAt)
	return nil
}

// Proxy is the core MCP interception proxy service.
type Proxy struct {
	sessions  *session.Manager
	policy    *policy.Engine
	simulator *simulate.Simulator
	detector  *fencedetect.Detector
	fenceCtrl *fence.Controller
	propBreak *propagation.Breaker
	auditor   *audit.Chain
	bus       *bus.Publisher
	log       *slog.Logger
}

// NewProxy creates and initializes a Proxy with all its dependencies.
func NewProxy(logger *slog.Logger) *Proxy {
	sessManager := session.NewManager()
	pe, _ := policy.NewEngine(context.Background(), "policy/agentfence", "v0.1.0")
	k8sSim := simulate.NewK8sSimulator()
	tfSim := simulate.NewTerraformSimulator()
	sim := simulate.NewSimulator(k8sSim, tfSim)
	detector := fencedetect.NewDetector(5 * time.Minute)
	eventBus := bus.NewPublisher(logger, "agentfence")
	auditChain := audit.NewChain()
	propBreaker := propagation.NewBreaker(false)
	freezeThreshold := 0.50
	quarantineThreshold := 0.75

	fenceCtrl := fence.NewController(
		&nilOrchestrator{},
		&sessionStoreAdapter{mgr: sessManager},
		&eventPublisherAdapter{pub: eventBus},
		freezeThreshold, quarantineThreshold, logger,
	)

	return &Proxy{
		sessions: sessManager, policy: pe, simulator: sim, detector: detector,
		fenceCtrl: fenceCtrl, propBreak: propBreaker,
		auditor: auditChain, bus: eventBus, log: logger,
	}
}

// HandleToolCall processes a tool call through the evaluation pipeline.
func (p *Proxy) HandleToolCall(ctx context.Context, sess *session.Session, evt ToolCallEvent) (Decision, error) {
	p.log.Info("handling tool call", "tool", evt.ToolName, "session", sess.ID)

	if sess.CurrentState() == session.StateFenced {
		return Decision{Outcome: "DENY", Reason: "session is fenced"}, nil
	}

	// Signal behavioral detection
	obs := fencedetect.Observation{
		ToolName: evt.ToolName, ToolServer: evt.ToolServer,
		IsWrite: isWriteTool(evt.ToolName),
		ArgEntropy: computeArgEntropy(evt.Arguments),
		ArgSize: len(evt.Arguments),
		DeclaredScope: sess.DeclaredScopeTags,
		TargetResource: extractTargetResource(evt.ToolName, evt.Arguments),
	}
	p.detector.Signal(sess.ID, obs)

	// Classify blast radius
	evt.BlastClass = p.policy.ClassifyBlastClass(evt.ToolName, evt.Arguments)

	// Simulate if high-risk
	var report *simulate.BlastRadiusReport
	if evt.BlastClass == "high" || evt.BlastClass == "critical" {
		var args map[string]interface{}
		if err := json.Unmarshal(evt.Arguments, &args); err == nil {
			r, err := p.simulator.Simulate(ctx, evt.ToolName, args)
			if err != nil {
				p.log.Error("simulation failed, denying", "err", err)
				return p.denyAndAudit(ctx, sess, evt, nil, "simulation error (fail-closed)")
			}
			report = r
		}
	}

	// Convert report to map for policy engine (which expects map type assertions)
	var blastReportInput interface{}
	if report != nil {
		blastReportInput = map[string]interface{}{
			"affected_resources": report.AffectedResources,
			"criticality_score":  float64(report.CriticalityScore),
			"destructive":        report.Destructive,
			"requires_approval":  report.RequiresApproval,
			"provider":           report.Provider,
		}
	}

	// Evaluate policy
	suspicion := p.detector.CurrentScore(sess.ID)
	snapshot := sess.Snapshot()
	outcome, reason, policyVersion, err := p.policy.Evaluate(ctx, policy.Input{
		Session: policy.SessionSnapshot{
			SessionID: snapshot.SessionID, AgentID: snapshot.AgentID,
			DeclaredScope: snapshot.DeclaredScope, State: snapshot.State, CallCount: snapshot.CallCount,
		},
		Tool: evt.ToolName, Arguments: evt.Arguments,
		BlastReport: blastReportInput, Suspicion: suspicion,
	})
	if err != nil {
		return p.denyAndAudit(ctx, sess, evt, report, "policy error (fail-closed)")
	}

	decision := Decision{
		DecisionID: newID(), Outcome: outcome, Reason: reason,
		BlastReport: report, SuspicionAtTime: suspicion, PolicyVersion: policyVersion,
	}

	// Audit and publish
	p.auditor.Append(ctx, evt.CallID, sess.ID, evt.ToolName, outcome, reason, suspicion)
	p.bus.PublishDecision(sess.ID, evt, decision)

	// Evaluate fence score
	p.fenceCtrl.EvaluateScore(ctx, sess.ID, suspicion)

	return decision, nil
}

func (p *Proxy) denyAndAudit(ctx context.Context, sess *session.Session, evt ToolCallEvent, report *simulate.BlastRadiusReport, reason string) (Decision, error) {
	d := Decision{DecisionID: newID(), Outcome: "DENY", Reason: reason, BlastReport: report}
	p.auditor.Append(ctx, evt.CallID, sess.ID, evt.ToolName, "DENY", reason, 0)
	p.bus.PublishDecision(sess.ID, evt, d)
	return d, nil
}

func (p *Proxy) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	var req EvaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	// Create a new session for this evaluation
	sess, err := p.sessions.CreateSession(r.Context(), agentID, "",
		req.DeclaredIntent, req.DeclaredScope)
	if err != nil {
		http.Error(w, `{"error":"failed to create session"}`, http.StatusInternalServerError)
		return
	}

	evt := ToolCallEvent{
		CallID:      newID(),
		SessionID:   sess.ID,
		ToolName:    req.ToolName,
		ToolServer:  req.ToolServer,
		Arguments:   req.Arguments,
		RequestedAt: time.Now(),
	}

	decision, err := p.HandleToolCall(r.Context(), sess, evt)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(EvaluateResponse{
		Decision:  decision,
		SessionID: sess.ID,
	})
}

func newID() string { return fmt.Sprintf("dec-%d", time.Now().UnixNano()) }

func isWriteTool(tool string) bool {
	writePrefixes := []string{
		"kubectl.delete", "kubectl.apply", "kubectl.patch",
		"terraform.apply", "terraform.destroy", "shell.exec", "fs.write", "fs.delete",
	}
	for _, p := range writePrefixes {
		if tool == p {
			return true
		}
	}
	return false
}

func computeArgEntropy(args json.RawMessage) float64 {
	if len(args) == 0 {
		return 0
	}
	var m map[string]interface{}
	if err := json.Unmarshal(args, &m); err != nil {
		return 0.3
	}
	for _, v := range m {
		if str, ok := v.(string); ok && len(str) > 100 {
			return 0.6
		}
	}
	return 0.1
}

func extractTargetResource(tool string, args json.RawMessage) string {
	var m map[string]interface{}
	if err := json.Unmarshal(args, &m); err != nil {
		return ""
	}
	if ns, ok := m["namespace"].(string); ok {
		return ns
	}
	if path, ok := m["path"].(string); ok {
		return path
	}
	if name, ok := m["name"].(string); ok {
		return name
	}
	return ""
}

type nilOrchestrator struct{}

func (n *nilOrchestrator) KillSession(ctx context.Context, sessionID string) error  { return nil }
func (n *nilOrchestrator) IsolateNetwork(ctx context.Context, sessionID string) error { return nil }

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Info("starting agentfence proxy", "version", "0.1.0")

	proxy := NewProxy(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "agentfence-proxy", "version": "0.1.0"})
	})
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"version": "0.1.0", "build": "dev"})
	})
	// WIRED: The evaluate endpoint routes incoming tool calls through the full pipeline
	mux.HandleFunc("/api/v1/evaluate", proxy.handleEvaluate)

	server := &http.Server{Addr: ":8443", Handler: mux}
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("shutting down proxy")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	logger.Info("agentfence proxy listening", "addr", ":8443")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}
}
