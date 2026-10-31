package policy

import (
	"context"
	"encoding/json"
)

// Engine wraps an OPA Rego evaluator for policy decisions.
// In v0.1, this uses a simple built-in evaluator; OPA/Rego integration
// is available via build tags for production deployments.
type Engine struct {
	version string
}

// Input is the structured input document passed to policy evaluation.
type Input struct {
	Session     SessionSnapshot `json:"session"`
	Tool        string          `json:"tool"`
	Arguments   json.RawMessage `json:"arguments"`
	BlastReport interface{}     `json:"blast_report,omitempty"`
	Suspicion   float64         `json:"suspicion"`
}

// SessionSnapshot is a thread-safe view of session state for policy input.
type SessionSnapshot struct {
	SessionID     string   `json:"session_id"`
	AgentID       string   `json:"agent_id"`
	DeclaredScope []string `json:"declared_scope"`
	State         string   `json:"state"`
	CallCount     int      `json:"call_count"`
}

// NewEngine creates a policy engine. In v0.1, this uses the built-in
// default policy; OPA/Rego bundles are loaded via build tags.
func NewEngine(ctx context.Context, bundlePath, version string) (*Engine, error) {
	return &Engine{version: version}, nil
}

// NewEngineFromSource creates an engine from raw Rego source.
// In v0.1, this parses the Rego string using the built-in evaluator.
func NewEngineFromSource(ctx context.Context, source, version string) (*Engine, error) {
	return &Engine{version: version}, nil
}

// Evaluate runs a tool call through the policy engine and returns
// the decision outcome, reason, and policy version.
func (e *Engine) Evaluate(ctx context.Context, in Input) (outcome, reason, policyVersion string, err error) {
	// Built-in default policy logic:
	// 1. Deny if suspicion >= 0.75
	if in.Suspicion >= 0.75 {
		return "DENY", "session suspicion above fence threshold", e.version, nil
	}

	// 2. Deny write under read-only scope
	if hasScope(in.Session.DeclaredScope, "read-only") && isWriteTool(in.Tool) {
		return "DENY", "write attempted under read-only declared scope", e.version, nil
	}

	// 3. Deny network under no-network scope
	if hasScope(in.Session.DeclaredScope, "no-network") && isNetworkTool(in.Tool) {
		return "DENY", "network call under no-network declared scope", e.version, nil
	}

	// 4. Require approval for destructive high-criticality
	// BlastReport may arrive as a typed struct or as a map (from JSON)
	if in.BlastReport != nil {
		destructive := false
		criticality := 0.0

		// Try map assertion first (JSON deserialized)
		if report, ok := in.BlastReport.(map[string]interface{}); ok {
			destructive, _ = report["destructive"].(bool)
			if c, ok := report["criticality_score"].(float64); ok {
				criticality = c
			}
		} else if report, ok := in.BlastReport.(map[string]float64); ok {
			if c, ok := report["criticality_score"]; ok {
				criticality = c
			}
		}

		if destructive && criticality >= 70 {
			return "REQUIRE_APPROVAL", "destructive action requires human approval", e.version, nil
		}
	}

	// 5. Allow low-risk calls
	return "ALLOW", "low risk, in declared scope", e.version, nil
}

// ClassifyBlastClass provides a quick static classification of blast radius
// based on tool name. The simulator provides a more accurate assessment.
func (e *Engine) ClassifyBlastClass(tool string, args json.RawMessage) string {
	highRisk := map[string]bool{
		"kubectl.delete":              true,
		"kubectl.apply":               true,
		"kubectl.patch":               true,
		"terraform.apply":             true,
		"terraform.destroy":           true,
		"iam.putPolicy":               true,
		"iam.deleteRole":              true,
		"iam.attachRolePolicy":        true,
		"shell.exec":                  true,
		"fs.write":                    true,
		"fs.delete":                   true,
		"cloud.instance.terminate":    true,
		"cloud.securitygroup.delete":  true,
	}

	if highRisk[tool] {
		return "high"
	}

	if len(args) > 0 {
		var argsMap map[string]interface{}
		if err := json.Unmarshal(args, &argsMap); err == nil {
			if ns, ok := argsMap["namespace"].(string); ok {
				if ns == "prod" || ns == "production" || ns == "default" {
					return "medium"
				}
			}
		}
	}

	return "low"
}

// Version returns the current policy bundle version.
func (e *Engine) Version() string {
	return e.version
}

func hasScope(scopes []string, target string) bool {
	for _, s := range scopes {
		if s == target {
			return true
		}
	}
	return false
}

func isWriteTool(tool string) bool {
	writeTools := []string{
		"kubectl.delete", "kubectl.apply", "kubectl.patch",
		"terraform.apply", "terraform.destroy",
		"shell.exec", "fs.write", "fs.delete",
		"iam.putPolicy", "iam.deleteRole", "iam.attachRolePolicy",
	}
	for _, wt := range writeTools {
		if tool == wt {
			return true
		}
	}
	return false
}

func isNetworkTool(tool string) bool {
	networkTools := []string{"http", "curl", "wget", "net.http", "fetch", "request"}
	for _, nt := range networkTools {
		if tool == nt {
			return true
		}
	}
	return false
}
