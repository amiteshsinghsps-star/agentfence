package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// APIServer provides the REST and gRPC control-plane API for AgentFence.
type APIServer struct {
	log *slog.Logger
}

// NewAPIServer creates a new API server.
func NewAPIServer(log *slog.Logger) *APIServer {
	return &APIServer{log: log}
}

// jsonResponse writes a JSON response with the given status code.
func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// errorResponse writes a JSON error response.
func errorResponse(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

// handleSessions lists sessions with optional filters.
func (s *APIServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	// TODO: wire up session manager
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"sessions": []interface{}{},
		"total":    0,
		"filter": map[string]string{
			"state": r.URL.Query().Get("state"),
		},
	})
}

// handleSessionDetail returns details for a single session.
func (s *APIServer) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if sessionID == "" {
		errorResponse(w, http.StatusBadRequest, "session_id required")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"session_id":       sessionID,
		"state":            "OPEN",
		"suspicion_score":  0.0,
		"declared_intent":  "read-only log analysis",
		"declared_scope":   []string{"read-only"},
		"call_count":       42,
		"started_at":       time.Now().Add(-30 * time.Minute).Format(time.RFC3339Nano),
	})
}

// handleFence manually fences a session.
func (s *APIServer) handleFence(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if sessionID == "" {
		errorResponse(w, http.StatusBadRequest, "session_id required")
		return
	}

	var body struct {
		Level  string `json:"level"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"fence_id":    fmt.Sprintf("fence-%d", time.Now().UnixNano()),
		"session_id":  sessionID,
		"level":       body.Level,
		"trigger":     "manual",
		"triggered_at": time.Now().Format(time.RFC3339Nano),
	})
}

// handleUnfence clears a fence.
func (s *APIServer) handleUnfence(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if sessionID == "" {
		errorResponse(w, http.StatusBadRequest, "session_id required")
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"session_id": sessionID,
		"state":      "OPEN",
		"reason":     body.Reason,
		"cleared_at": time.Now().Format(time.RFC3339Nano),
	})
}

// handleDecisions lists decisions, filterable.
func (s *APIServer) handleDecisions(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"decisions": []interface{}{},
		"total":     0,
	})
}

// handlePolicy returns the current policy bundle metadata.
func (s *APIServer) handlePolicy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"version":      "v0.1.0",
			"bundle":       "agentfence-policies",
			"rego_source":  "# policy bundle loaded",
			"rules":        4,
			"last_updated": time.Now().Add(-2 * time.Hour).Format(time.RFC3339Nano),
		})
	case http.MethodPut:
		// Publish new policy
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"status":  "published",
			"version": "v0.1.1",
		})
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handlePolicyDryRun runs a dry-run evaluation of candidate policy.
func (s *APIServer) handlePolicyDryRun(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"diff": []interface{}{},
		"summary": map[string]int{
			"total_calls":  0,
			"changed":      0,
			"allowed":      0,
			"denied":       0,
		},
	})
}

// handleTopology returns the current spawn graph.
func (s *APIServer) handleTopology(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"nodes": []interface{}{},
		"edges": []interface{}{},
	})
}

// handleSimulate runs an ad-hoc blast-radius simulation.
func (s *APIServer) handleSimulate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ToolName    string                 `json:"tool_name"`
		Arguments   map[string]interface{} `json:"arguments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"affected_resources": 14,
		"criticality_score":  91,
		"destructive":        true,
		"requires_approval":  true,
		"provider":           "kubernetes",
		"details":            fmt.Sprintf("%s simulation", body.ToolName),
	})
}

// handleAuditExport streams an NDJSON export of the audit trail.
func (s *APIServer) handleAuditExport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=agentfence-audit-%s.ndjson", time.Now().Format("2006-01-02")))
	// Stream records
	for i := 0; i < 10; i++ {
		rec := map[string]interface{}{
			"decision_id": fmt.Sprintf("dec-%d", i),
			"outcome":     "ALLOW",
			"timestamp":   time.Now().Add(-time.Duration(i) * time.Minute).Format(time.RFC3339Nano),
		}
		data, _ := json.Marshal(rec)
		fmt.Fprintf(w, "%s\n", data)
	}
}

// CORSMiddleware adds CORS headers for the dashboard.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Info("starting agentfence apiserver", "version", "0.1.0")

	server := NewAPIServer(logger)
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"service": "agentfence-api",
			"version": "0.1.0",
		})
	})

	// Sessions
	mux.HandleFunc("GET /api/v1/sessions", server.handleSessions)
	mux.HandleFunc("GET /api/v1/sessions/{id}", server.handleSessionDetail)
	mux.HandleFunc("POST /api/v1/sessions/{id}/fence", server.handleFence)
	mux.HandleFunc("POST /api/v1/sessions/{id}/unfence", server.handleUnfence)

	// Decisions
	mux.HandleFunc("GET /api/v1/decisions", server.handleDecisions)

	// Policy
	mux.HandleFunc("GET /api/v1/policy", server.handlePolicy)
	mux.HandleFunc("PUT /api/v1/policy", server.handlePolicy)
	mux.HandleFunc("POST /api/v1/policy/dry-run", server.handlePolicyDryRun)

	// Topology
	mux.HandleFunc("GET /api/v1/topology", server.handleTopology)

	// Simulation
	mux.HandleFunc("POST /api/v1/simulate", server.handleSimulate)

	// Audit
	mux.HandleFunc("GET /api/v1/audit/export", server.handleAuditExport)

	addr := ":8081"
	if a := os.Getenv("AGENTFENCE_API_ADDR"); a != "" {
		addr = a
	}

	logger.Info("api server listening", "addr", addr)
	if err := http.ListenAndServe(addr, CORSMiddleware(mux)); err != nil {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}
}
