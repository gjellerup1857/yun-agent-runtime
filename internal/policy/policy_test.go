package policy

import (
	"testing"

	"github.com/gjellerup1857/yun-agent-runtime/internal/tools"
)

func TestDefaultDeny(t *testing.T) {
	engine := New(map[string]AgentPolicy{
		"backend": {Allow: map[string]struct{}{}},
	})
	result := engine.Evaluate("backend", tools.Tool{ID: "repo.write", Risk: tools.RiskLow})
	if result.Allowed {
		t.Fatal("expected default deny")
	}
}

func TestApprovalRequired(t *testing.T) {
	engine := New(map[string]AgentPolicy{
		"backend": {
			ApprovalRequired: map[string]struct{}{"repo.write": {}},
		},
	})
	result := engine.Evaluate("backend", tools.Tool{ID: "repo.write", Risk: tools.RiskMedium})
	if !result.Allowed || !result.RequiresApproval {
		t.Fatalf("expected approval-required allow, got %+v", result)
	}
}

func TestCriticalRiskDenied(t *testing.T) {
	engine := New(map[string]AgentPolicy{
		"backend": {Allow: map[string]struct{}{"prod.delete": {}}},
	})
	result := engine.Evaluate("backend", tools.Tool{ID: "prod.delete", Risk: tools.RiskCritical})
	if result.Allowed {
		t.Fatal("critical-risk tool must be denied")
	}
}
