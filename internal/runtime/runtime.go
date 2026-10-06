package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/gjellerup1857/yun-agent-runtime/internal/provider"
	"github.com/gjellerup1857/yun-agent-runtime/internal/routing"
)

type Runtime struct {
	router   *routing.Router
	provider provider.Provider
}

type RunRequest struct {
	ProjectID string `json:"project_id,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
	Message   string `json:"message"`
}

type AgentResult struct {
	AgentID string            `json:"agent_id"`
	Output  string            `json:"output"`
	Usage   provider.Response `json:"usage"`
}

type RunResponse struct {
	AgentsUsed []string      `json:"agents_used"`
	Results    []AgentResult `json:"results"`
	Answer     string        `json:"answer"`
}

func New(router *routing.Router, p provider.Provider) *Runtime {
	return &Runtime{router: router, provider: p}
}

func (r *Runtime) Run(ctx context.Context, req RunRequest) (RunResponse, error) {
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return RunResponse{}, fmt.Errorf("message is required")
	}

	agents := r.router.Route(message)
	results := make([]AgentResult, 0, len(agents))
	var answer strings.Builder

	for i, agentID := range agents {
		resp, err := r.provider.Generate(ctx, provider.Request{
			Model:  "mock-balanced",
			System: "You are " + agentID + " in Yun Agent Runtime.",
			Input:  message,
		})
		if err != nil {
			return RunResponse{}, err
		}
		results = append(results, AgentResult{AgentID: agentID, Output: resp.Text, Usage: resp})
		if i > 0 {
			answer.WriteString("\n\n")
		}
		answer.WriteString("[")
		answer.WriteString(agentID)
		answer.WriteString("]\n")
		answer.WriteString(resp.Text)
	}

	return RunResponse{AgentsUsed: agents, Results: results, Answer: answer.String()}, nil
}
