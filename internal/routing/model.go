package routing

import "github.com/gjellerup1857/yun-agent-runtime/internal/provider"

type ModelRouter struct {
	models map[string]provider.ModelRef
}

func NewModelRouter(models map[string]provider.ModelRef) *ModelRouter {
	copy := make(map[string]provider.ModelRef, len(models))
	for id, model := range models {
		copy[id] = model
	}
	return &ModelRouter{models: copy}
}

func (r *ModelRouter) Route(agentID string) []provider.ModelRef {
	var order []string
	switch agentID {
	case "frontend-engineer", "fullstack-engineer", "backend-engineer", "automation-test-engineer":
		order = []string{"anthropic", "openai", "google", "mock"}
	case "software-architect", "ai-engineer", "coordinator", "technical-product-manager":
		order = []string{"openai", "anthropic", "google", "mock"}
	default:
		order = []string{"google", "openai", "anthropic", "mock"}
	}

	result := make([]provider.ModelRef, 0, len(order))
	for _, id := range order {
		if model, ok := r.models[id]; ok && model.Model != "" {
			result = append(result, model)
		}
	}
	return result
}
