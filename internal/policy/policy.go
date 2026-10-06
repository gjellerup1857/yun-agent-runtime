package policy

import "github.com/gjellerup1857/yun-agent-runtime/internal/tools"

type Decision struct {
	Allowed bool
	RequiresApproval bool
	Reason string
}

type AgentPolicy struct {
	Allow map[string]struct{}
	ApprovalRequired map[string]struct{}
	Deny map[string]struct{}
}

type Engine struct { policies map[string]AgentPolicy }

func New(policies map[string]AgentPolicy) *Engine { return &Engine{policies: policies} }

func (e *Engine) Evaluate(agentID string, tool tools.Tool) Decision {
	p, ok := e.policies[agentID]
	if !ok {
		return Decision{Reason: "agent has no tool policy"}
	}
	if _, ok := p.Deny[tool.ID]; ok {
		return Decision{Reason: "tool explicitly denied"}
	}
	if tool.Risk == tools.RiskCritical {
		return Decision{Reason: "critical-risk tool denied"}
	}
	if _, ok := p.ApprovalRequired[tool.ID]; ok {
		return Decision{Allowed: true, RequiresApproval: true, Reason: "user approval required"}
	}
	if _, ok := p.Allow[tool.ID]; ok {
		return Decision{Allowed: true, Reason: "explicitly allowed"}
	}
	return Decision{Reason: "default deny"}
}

func (e *Engine) CanDiscover(agentID string, tool tools.Tool) bool {
	d := e.Evaluate(agentID, tool)
	return d.Allowed
}
