package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/gjellerup1857/yun-agent-runtime/internal/inference"
	"github.com/gjellerup1857/yun-agent-runtime/internal/memory"
	"github.com/gjellerup1857/yun-agent-runtime/internal/policy"
	"github.com/gjellerup1857/yun-agent-runtime/internal/provider"
	"github.com/gjellerup1857/yun-agent-runtime/internal/routing"
	"github.com/gjellerup1857/yun-agent-runtime/internal/task"
	"github.com/gjellerup1857/yun-agent-runtime/internal/toolruntime"
	"github.com/gjellerup1857/yun-agent-runtime/internal/tools"
)

const (
	defaultTeamID     = "yun-product-engineering"
	maxToolRounds     = 6
	maxToolsPerRound  = 4
	maxTotalToolCalls = 12
)

type Runtime struct {
	router       *routing.Router
	modelRouter  *routing.ModelRouter
	inference    *inference.Service
	memories     memory.Repository
	tasks        task.Repository
	extractor    *memory.Extractor
	toolRegistry *tools.Registry
	policyEngine *policy.Engine
	toolLoop     *toolLoop
}

type RunRequest struct {
	TenantID  string `json:"-"`
	UserID    string `json:"-"`
	ProjectID string `json:"project_id,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
	Message   string `json:"message"`
}

type AgentResult struct {
	AgentID  string         `json:"agent_id"`
	Provider string         `json:"provider"`
	Model    string         `json:"model"`
	Output   string         `json:"output"`
	Usage    provider.Usage `json:"usage"`
}

type RunResponse struct {
	RunID             string        `json:"run_id,omitempty"`
	TaskID            string        `json:"task_id,omitempty"`
	ProjectID         string        `json:"project_id,omitempty"`
	AgentsUsed        []string      `json:"agents_used"`
	Results           []AgentResult `json:"results"`
	Answer            string        `json:"answer"`
	MemoriesRead      int           `json:"memories_read,omitempty"`
	MemoriesWritten   int           `json:"memories_written,omitempty"`
	PendingApprovalID string        `json:"pending_approval_id,omitempty"`
}

func New(router *routing.Router, p provider.Provider) *Runtime {
	registry := provider.NewRegistry()
	registry.Register(p)
	modelID := p.Name()
	modelName := "mock-balanced"
	if p.Name() != "mock" {
		modelName = "test-model"
	}
	models := map[string]provider.ModelRef{
		modelID: {ID: modelID, Provider: p.Name(), Model: modelName},
	}
	if p.Name() != "mock" {
		models["mock"] = provider.ModelRef{ID: "mock", Provider: p.Name(), Model: modelName}
	}
	return NewMulti(router, routing.NewModelRouter(models), inference.New(registry), nil, nil, nil)
}

func NewStateful(router *routing.Router, p provider.Provider, memories memory.Repository, tasks task.Repository, extractor *memory.Extractor) *Runtime {
	registry := provider.NewRegistry()
	registry.Register(p)
	models := map[string]provider.ModelRef{
		"mock": {ID: "mock", Provider: p.Name(), Model: "mock-balanced"},
	}
	return NewMulti(router, routing.NewModelRouter(models), inference.New(registry), memories, tasks, extractor)
}

func NewMulti(
	router *routing.Router,
	modelRouter *routing.ModelRouter,
	inferenceService *inference.Service,
	memories memory.Repository,
	tasks task.Repository,
	extractor *memory.Extractor,
) *Runtime {
	return &Runtime{
		router:      router,
		modelRouter: modelRouter,
		inference:   inferenceService,
		memories:    memories,
		tasks:       tasks,
		extractor:   extractor,
	}
}

func (r *Runtime) WithTools(registry *tools.Registry, executor *toolruntime.Executor, policies *policy.Engine) *Runtime {
	r.toolRegistry = registry
	r.policyEngine = policies
	r.toolLoop = newToolLoop(executor)
	return r
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
				TenantID:       req.TenantID,
				UserID:         req.UserID,
				TeamID:         defaultTeamID,
				ProjectID:      req.ProjectID,
				AgentID:        agentID,
				TaskID:         req.TaskID,
				MinImportance:  0.6,
				Limit:          12,
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

		agentResult, err := r.runAgent(ctx, req, agentID, system)
		if err != nil {
			var waiting *waitingApprovalError
			if errors.As(err, &waiting) && stateful {
				_ = r.tasks.UpdateStatus(ctx, req.TaskID, task.StatusWaitingApproval)
				return RunResponse{
					RunID:             uuid.NewString(),
					TaskID:            req.TaskID,
					ProjectID:         req.ProjectID,
					AgentsUsed:        agents[:i+1],
					Results:           results,
					Answer:            "Task is waiting for approval.",
					MemoriesRead:      memoriesRead,
					PendingApprovalID: waiting.ApprovalID,
				}, nil
			}
			return RunResponse{}, err
		}

		results = append(results, agentResult)
		if i > 0 {
			answer.WriteString("\n\n")
		}
		answer.WriteString("[")
		answer.WriteString(agentID)
		answer.WriteString("]\n")
		answer.WriteString(agentResult.Output)
	}

	written := 0
	if stateful {
		for _, candidate := range r.extractor.Extract(memory.ExtractRequest{
			TenantID:  req.TenantID,
			UserID:    req.UserID,
			TeamID:    defaultTeamID,
			ProjectID: req.ProjectID,
			TaskID:    req.TaskID,
			Message:   message,
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
		RunID:           uuid.NewString(),
		TaskID:          currentTask.ID,
		ProjectID:       req.ProjectID,
		AgentsUsed:      agents,
		Results:         results,
		Answer:          answer.String(),
		MemoriesRead:    memoriesRead,
		MemoriesWritten: written,
	}, nil
}

func (r *Runtime) runAgent(ctx context.Context, req RunRequest, agentID, system string) (AgentResult, error) {
	candidates := r.modelRouter.Route(agentID)
	if len(candidates) == 0 {
		return AgentResult{}, fmt.Errorf("no model candidate for agent %s", agentID)
	}

	modelReq := provider.Request{
		System:          system,
		Input:           req.Message,
		MaxOutputTokens: 4096,
	}
	if r.toolRegistry != nil && r.policyEngine != nil {
		modelReq.Tools = r.toolRegistry.DefinitionsForAgent(agentID, r.policyEngine.CanDiscover)
	}

	var totalUsage provider.Usage
	totalToolCalls := 0
	var selectedModel provider.ModelRef

	for round := 0; round < maxToolRounds; round++ {
		response, model, err := r.inference.Generate(ctx, candidates, modelReq)
		if err != nil {
			return AgentResult{}, err
		}
		selectedModel = model
		totalUsage.InputTokens += response.Usage.InputTokens
		totalUsage.OutputTokens += response.Usage.OutputTokens
		totalUsage.ReasoningTokens += response.Usage.ReasoningTokens
		totalUsage.TotalTokens += response.Usage.TotalTokens

		if len(response.ToolCalls) == 0 {
			return AgentResult{
				AgentID:  agentID,
				Provider: selectedModel.Provider,
				Model:    selectedModel.Model,
				Output:   response.Text,
				Usage:    totalUsage,
			}, nil
		}
		if r.toolLoop == nil {
			return AgentResult{}, fmt.Errorf("agent %s requested tools but tool runtime is unavailable", agentID)
		}
		if len(response.ToolCalls) > maxToolsPerRound {
			return AgentResult{}, fmt.Errorf("tool budget exceeded: agent %s requested %d tools in one round", agentID, len(response.ToolCalls))
		}
		totalToolCalls += len(response.ToolCalls)
		if totalToolCalls > maxTotalToolCalls {
			return AgentResult{}, fmt.Errorf("tool budget exceeded: agent %s requested more than %d tools", agentID, maxTotalToolCalls)
		}

		toolResults, err := r.toolLoop.executeCalls(ctx, req, agentID, response.ToolCalls)
		if err != nil {
			return AgentResult{}, err
		}
		modelReq.ToolResults = append(modelReq.ToolResults, toolResults...)
	}

	return AgentResult{}, fmt.Errorf("agent %s exceeded maximum tool rounds", agentID)
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
		TenantID:  req.TenantID,
		UserID:    req.UserID,
		TeamID:    defaultTeamID,
		ProjectID: req.ProjectID,
		Title:     title,
		Status:    task.StatusRunning,
	})
}
