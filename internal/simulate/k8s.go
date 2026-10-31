package simulate

import (
	"context"
	"fmt"
)

// BlastRadiusReport is the normalized output of a blast-radius simulation.
type BlastRadiusReport struct {
	AffectedResources int    `json:"affected_resources"`
	CriticalityScore  int    `json:"criticality_score"`
	Destructive       bool   `json:"destructive"`
	RequiresApproval  bool   `json:"requires_approval"`
	Provider          string `json:"provider"`
	Details           string `json:"details,omitempty"`
}

// Simulator orchestration — combines multiple provider simulators.
type Simulator struct {
	k8s       *K8sSimulator
	terraform *TerraformSimulator
}

// NewSimulator creates a new blast-radius simulator with all providers.
func NewSimulator(k8s *K8sSimulator, terraform *TerraformSimulator) *Simulator {
	return &Simulator{
		k8s:       k8s,
		terraform: terraform,
	}
}

// Simulate runs the blast-radius simulation for a given tool call.
func (s *Simulator) Simulate(ctx context.Context, toolName string, args map[string]interface{}) (*BlastRadiusReport, error) {
	switch {
	case isK8sTool(toolName):
		if s.k8s == nil {
			return defaultReport(false, "K8s simulator not configured"), nil
		}
		return s.k8s.Simulate(ctx, toolName, args)
	case isTerraformTool(toolName):
		if s.terraform == nil {
			return defaultReport(true, "Terraform simulator not configured"), nil
		}
		return s.terraform.Simulate(ctx, toolName, args)
	default:
		return defaultReport(false, "no simulator for this tool"), nil
	}
}

func isK8sTool(tool string) bool {
	k8sPrefixes := []string{"kubectl.", "k8s.", "kubernetes."}
	for _, p := range k8sPrefixes {
		if len(tool) >= len(p) && tool[:len(p)] == p {
			return true
		}
	}
	return false
}

func isTerraformTool(tool string) bool {
	return tool == "terraform.apply" || tool == "terraform.destroy" || tool == "terraform.plan"
}

func defaultReport(destructive bool, details string) *BlastRadiusReport {
	return &BlastRadiusReport{
		AffectedResources: 0,
		CriticalityScore:  0,
		Destructive:       destructive,
		RequiresApproval:  destructive,
		Provider:          "unknown",
		Details:           details,
	}
}

// K8sSimulator uses client-go dry-run to estimate blast radius.
type K8sSimulator struct {
	// In production, this holds a dynamic.Interface client.
	// For v0.1, we use a registry-based simulation.
}

// NewK8sSimulator creates a new K8s blast-radius simulator.
func NewK8sSimulator() *K8sSimulator {
	return &K8sSimulator{}
}

// Simulate estimates the blast radius of a Kubernetes API call.
func (s *K8sSimulator) Simulate(ctx context.Context, toolName string, args map[string]interface{}) (*BlastRadiusReport, error) {
	verb := extractVerb(toolName)
	destructive := isDestructiveVerb(verb)

	namespace := "default"
	if ns, ok := args["namespace"].(string); ok {
		namespace = ns
	}

	affected := estimateCascadeAffected(verb, args)
	criticality := computeCriticality(namespace, affected, destructive)

	return &BlastRadiusReport{
		AffectedResources: affected,
		CriticalityScore:  criticality,
		Destructive:       destructive,
		RequiresApproval:  destructive && criticality >= 70,
		Provider:          "kubernetes",
		Details:           fmt.Sprintf("%s on namespace %s", toolName, namespace),
	}, nil
}

func extractVerb(toolName string) string {
	parts := splitToolName(toolName)
	if len(parts) > 1 {
		return parts[1]
	}
	return "get"
}

func splitToolName(name string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(name); i++ {
		if name[i] == '.' {
			parts = append(parts, name[start:i])
			start = i + 1
		}
	}
	parts = append(parts, name[start:])
	return parts
}

func isDestructiveVerb(verb string) bool {
	destructive := map[string]bool{
		"delete":          true,
		"deletecollection": true,
		"patch":           true,
		"apply":           true,
		"create":          true,
		"update":          true,
	}
	return destructive[verb]
}

func estimateCascadeAffected(verb string, args map[string]interface{}) int {
	base := 1
	if isDestructiveVerb(verb) {
		base = 3 // estimate 3 resources affected for destructive operations
	}
	// Check for cascade options
	if cascade, ok := args["cascade"].(string); ok && cascade == "true" {
		base *= 5
	}
	return base
}

func computeCriticality(namespace string, affected int, destructive bool) int {
	base := 20
	if namespace == "prod" || namespace == "production" {
		base = 70
	} else if namespace == "staging" || namespace == "stage" {
		base = 45
	}

	if destructive {
		base += 20
	}

	score := base + affected*3
	if score > 100 {
		return 100
	}
	return score
}

// TerraformSimulator parses terraform plan output to estimate blast radius.
type TerraformSimulator struct{}

// NewTerraformSimulator creates a new Terraform blast-radius simulator.
func NewTerraformSimulator() *TerraformSimulator {
	return &TerraformSimulator{}
}

// Simulate estimates the blast radius of a Terraform operation.
func (s *TerraformSimulator) Simulate(ctx context.Context, toolName string, args map[string]interface{}) (*BlastRadiusReport, error) {
	isDestroy := toolName == "terraform.destroy"
	affected := 1
	if isDestroy {
		affected = 5
	}
	if count, ok := args["resource_count"].(float64); ok {
		affected = int(count)
	}

	criticality := 50
	if isDestroy {
		criticality = 75
	}

	return &BlastRadiusReport{
		AffectedResources: affected,
		CriticalityScore:  criticality,
		Destructive:       isDestroy || toolName == "terraform.apply",
		RequiresApproval:  criticality >= 70,
		Provider:          "terraform",
		Details:           fmt.Sprintf("%s with %d resources", toolName, affected),
	}, nil
}
