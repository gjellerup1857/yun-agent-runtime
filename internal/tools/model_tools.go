package tools

import "github.com/gjellerup1857/yun-agent-runtime/internal/provider"

func (r *Registry) DefinitionsForAgent(
	agentID string,
	canDiscover func(agentID string, tool Tool) bool,
) []provider.ToolDefinition {
	all := r.List()
	result := make([]provider.ToolDefinition, 0, len(all))
	for _, tool := range all {
		if !canDiscover(agentID, tool) {
			continue
		}
		result = append(result, provider.ToolDefinition{
			Name:        tool.ID,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	return result
}
