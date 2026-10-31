package simulate

import (
	"context"
	"testing"
)

func TestK8sSimulator_DeleteProd(t *testing.T) {
	s := NewK8sSimulator()
	report, err := s.Simulate(context.Background(), "kubectl.delete", map[string]interface{}{
		"namespace": "prod",
		"name":     "checkout-api",
	})
	if err != nil {
		t.Fatalf("Simulate failed: %v", err)
	}
	if !report.Destructive {
		t.Error("expected Destructive=true for delete")
	}
	if report.CriticalityScore < 70 {
		t.Errorf("expected high criticality for prod delete, got %d", report.CriticalityScore)
	}
	if !report.RequiresApproval {
		t.Error("expected RequiresApproval for destructive prod action")
	}
	if report.Provider != "kubernetes" {
		t.Errorf("expected provider kubernetes, got %s", report.Provider)
	}
}

func TestK8sSimulator_GetDev(t *testing.T) {
	s := NewK8sSimulator()
	report, err := s.Simulate(context.Background(), "kubectl.get", map[string]interface{}{
		"namespace": "dev",
	})
	if err != nil {
		t.Fatalf("Simulate failed: %v", err)
	}
	if report.Destructive {
		t.Error("expected Destructive=false for get")
	}
	if report.AffectedResources < 1 {
		t.Errorf("expected at least 1 affected resource, got %d", report.AffectedResources)
	}
}

func TestK8sSimulator_StagingNamespace(t *testing.T) {
	s := NewK8sSimulator()
	report, _ := s.Simulate(context.Background(), "kubectl.delete", map[string]interface{}{
		"namespace": "staging",
	})
	if report.CriticalityScore < 45 {
		t.Errorf("expected criticality >=45 for staging, got %d", report.CriticalityScore)
	}
}

func TestK8sSimulator_CascadeDelete(t *testing.T) {
	s := NewK8sSimulator()
	report, _ := s.Simulate(context.Background(), "kubectl.delete", map[string]interface{}{
		"namespace": "default",
		"cascade":   "true",
	})
	if report.AffectedResources < 5 {
		t.Errorf("expected >=5 resources for cascade delete, got %d", report.AffectedResources)
	}
}

func TestTerraformSimulator_Destroy(t *testing.T) {
	s := NewTerraformSimulator()
	report, err := s.Simulate(context.Background(), "terraform.destroy", nil)
	if err != nil {
		t.Fatalf("Simulate failed: %v", err)
	}
	if !report.Destructive {
		t.Error("expected Destructive=true for destroy")
	}
	if report.CriticalityScore < 70 {
		t.Errorf("expected criticality >=70 for destroy, got %d", report.CriticalityScore)
	}
	if report.Provider != "terraform" {
		t.Errorf("expected provider terraform, got %s", report.Provider)
	}
}

func TestTerraformSimulator_Plan(t *testing.T) {
	s := NewTerraformSimulator()
	report, _ := s.Simulate(context.Background(), "terraform.plan", map[string]interface{}{
		"resource_count": 15.0,
	})
	if report.AffectedResources != 15 {
		t.Errorf("expected 15 affected resources, got %d", report.AffectedResources)
	}
}

func TestSimulator_Dispatch(t *testing.T) {
	k8s := NewK8sSimulator()
	tf := NewTerraformSimulator()
	sim := NewSimulator(k8s, tf)

	// K8s tool
	report, err := sim.Simulate(context.Background(), "kubectl.get", nil)
	if err != nil {
		t.Fatalf("sim.Simulate(k8s) failed: %v", err)
	}
	if report.Provider != "kubernetes" {
		t.Errorf("expected kubernetes, got %s", report.Provider)
	}

	// Terraform tool
	report, _ = sim.Simulate(context.Background(), "terraform.apply", nil)
	if report.Provider != "terraform" {
		t.Errorf("expected terraform, got %s", report.Provider)
	}

	// Unknown tool
	report, _ = sim.Simulate(context.Background(), "unknown.tool", nil)
	if report.Provider != "unknown" {
		t.Errorf("expected unknown provider, got %s", report.Provider)
	}
}

func TestSimulator_NilSimulators(t *testing.T) {
	sim := NewSimulator(nil, nil)
	report, err := sim.Simulate(context.Background(), "kubectl.get", nil)
	if err != nil {
		t.Fatalf("Simulate with nil simulators failed: %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}
}

func TestExtractVerb(t *testing.T) {
	tests := []struct {
		tool string
		want string
	}{
		{"kubectl.delete", "delete"},
		{"kubectl.apply", "apply"},
		{"fs.read", "read"},
		{"unknown", "get"},  // no dot → defaults to "get"
		{"a.b.c", "b"},
	}
	for _, tt := range tests {
		got := extractVerb(tt.tool)
		if got != tt.want {
			t.Errorf("extractVerb(%q) = %q, want %q", tt.tool, got, tt.want)
		}
	}
}

func TestIsDestructiveVerb(t *testing.T) {
	if !isDestructiveVerb("delete") {
		t.Error("expected delete to be destructive")
	}
	if !isDestructiveVerb("apply") {
		t.Error("expected apply to be destructive")
	}
	if isDestructiveVerb("get") {
		t.Error("expected get to NOT be destructive")
	}
	if isDestructiveVerb("list") {
		t.Error("expected list to NOT be destructive")
	}
}
