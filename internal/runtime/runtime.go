package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/gjellerup1857/yun-agent-runtime/internal/memory"
	"github.com/gjellerup1857/yun-agent-runtime/internal/provider"
	"github.com/gjellerup1857/yun-agent-runtime/internal/routing"
	"github.com/gjellerup1857/yun-agent-runtime/internal/task"
)

const defaultTeamID = "yun-product-engineering"

type Runtime struct {
	router    *routing.Router
	provider  provider.Provider
	memories  memory.Repository
	tasks     task.Repository
	extractor *memory.Extractor
}

type RunRequest struct {
	TenantID  string `json:"-"`
	UserID    string `json:"-"`
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
	RunID           string        `json:"run_id,omitempty"`
	TaskID          string        `json:"task_id,omitempty"`
	ProjectID       string        `json:"project_id,omitempty"`
	AgentsUsed      []string      `json:"agents_used"`
	Results         []AgentResult `json:"results"`
	Answer          string        `json:"answer"`
	MemoriesRead    int           `json:"memories_read,omitempty"`
	MemoriesWritten int           `json:"memories_written,omitempty"`
}

func New(router *routing.Router, p provider.Provider) *Runtime {
	return &Runtime{router: router, provider: p}
}

func NewStateful(router *routing.Router, p provider.Provider, memories memory.Repository, tasks task.Repository, extractor *memory.Extractor) *Runtime {
	return &Runtime{router: router, provider: p, memories: memories, tasks: tasks, extractor: extractor}
}

func (r *Runtime) Run(ctx context.Context, req RunRequest) (RunResponse, error) {
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return RunResponse{}, fmt.Errorf("message is required")
	}

	var currentTask task.Task
	var err error
	stateful := r.memories != nil && r.tasks != nil && r.extractor != nil
	if stateful {
		if req.TenantID == "" || req.UserID == "" {
			return RunResponse{}, fmt.Errorf("tenant and user identity are required")
		}
		currentTask, err = r.resolveTask(ctx, req)
		if err != nil {
			return RunResponse{}, err
		}
		req.TaskID = currentTask.ID
	}

	agents := r.router.Route(message)
	results := make([]AgentResult, 0, len(agents))
	var answer strings.Builder
	memoriesRead := 0

	for i, agentID := range agents {
		system := "You are " + agentID + " in Yun Agent Runtime."
		if stateful {
			relevant, err := r.memories.Relevant(ctx, memory.Query{
				TenantID: req.TenantID,
				UserID: req.UserID,
				TeamID: defaultTeamID,
				ProjectID: req.ProjectID,
				AgentID: agentID,
				TaskID: req.TaskID,
				MinImportance: 0.6,
				Limit: 12,
			})
			if err != nil {
				return RunResponse{}, err
			}
			memoriesRead += len(relevant)
			if len(relevant) > 0 {
				var b strings.Builder
				b.WriteString(system)
				b.WriteString("\nRelevant canonical memory:\n")
				for _, item := range relevant {
					b.WriteString("- [")
					b.WriteString(string(item.Type))
					b.WriteString("] ")
					b.WriteString(item.Content)
					b.WriteString("\n")
				}
				system = b.String()
			}
		}

		resp, err := r.provider.Generate(ctx, provider.Request{
			Model: "mock-balanced",
			System: system,
			Input: message,
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

	written := 0
	if stateful {
		for _, candidate := range r.extractor.Extract(memory.ExtractRequest{
			TenantID: req.TenantID,
			UserID: req.UserID,
			TeamID: defaultTeamID,
			ProjectID: req.ProjectID,
			TaskID: req.TaskID,
			Message: message,
		}) {
			if candidate.Type == memory.TypeDecision {
				_, err = r.memories.SaveDecision(ctx, candidate)
			} else {
				_, err = r.memories.Save(ctx, candidate)
			}
			if err != nil {
				return RunResponse{}, err
			}
			written++
		}
	}

	return RunResponse{
		RunID: uuid.NewString(),
		TaskID: currentTask.ID,
		ProjectID: req.ProjectID,
		AgentsUsed: agents,
		Results: results,
		Answer: answer.String(),
		MemoriesRead: memoriesRead,
		MemoriesWritten: written,
	}, nil
}

func (r *Runtime) resolveTask(ctx context.Context, req RunRequest) (task.Task, error) {
	if req.TaskID != "" {
		return r.tasks.Get(ctx, req.TenantID, req.UserID, req.TaskID)
	}
	title := req.Message
	runes := []rune(title)
	if len(runes) > 80 {
		title = string(runes[:80])
	}
	return r.tasks.Create(ctx, task.Task{
		TenantID: req.TenantID,
		UserID: req.UserID,
		TeamID: defaultTeamID,
		ProjectID: req.ProjectID,
		Title: title,
		Status: task.StatusRunning,
	})
}
