package toolruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gjellerup1857/yun-agent-runtime/internal/approval"
	"github.com/gjellerup1857/yun-agent-runtime/internal/audit"
	"github.com/gjellerup1857/yun-agent-runtime/internal/policy"
	"github.com/gjellerup1857/yun-agent-runtime/internal/tools"
)

type Executor struct {
	registry *tools.Registry
	validator tools.SchemaValidator
	policies *policy.Engine
	approvals *approval.Service
	executions tools.ExecutionRepository
	audit audit.Repository
	backends map[string]tools.Backend
}

func NewExecutor(
	registry *tools.Registry,
	validator tools.SchemaValidator,
	policies *policy.Engine,
	approvals *approval.Service,
	executions tools.ExecutionRepository,
	auditRepo audit.Repository,
	backends []tools.Backend,
) *Executor {
	backendMap := make(map[string]tools.Backend, len(backends))
	for _, backend := range backends {
		backendMap[backend.Name()] = backend
	}
	return &Executor{
		registry: registry,
		validator: validator,
		policies: policies,
		approvals: approvals,
		executions: executions,
		audit: auditRepo,
		backends: backendMap,
	}
}

func (e *Executor) Execute(ctx context.Context, inv tools.Invocation) (tools.Result, error) {
	tool, ok := e.registry.Get(inv.ToolID)
	if !ok {
		return tools.Result{}, tools.ErrToolNotFound
	}
	if err := e.validator.Validate(tool.InputSchema, inv.Arguments); err != nil {
		return tools.Result{}, err
	}

	decision := e.policies.Evaluate(inv.AgentID, tool)
	argumentsHash := hashArguments(inv.Arguments)
	if !decision.Allowed {
		_ = e.audit.Record(ctx, audit.Event{
			TenantID: inv.TenantID,
			UserID: inv.UserID,
			TaskID: inv.TaskID,
			AgentID: inv.AgentID,
			EventType: "tool_denied",
			ResourceType: "tool",
			ResourceID: tool.ID,
			Action: string(tool.Operation),
			Result: "denied",
			PayloadHash: argumentsHash,
		})
		return tools.Result{}, fmt.Errorf("%w: %s", tools.ErrToolDenied, decision.Reason)
	}

	writeLike := tool.Operation == tools.OperationWrite || tool.Operation == tools.OperationExecute || tool.Operation == tools.OperationDestructive
	if writeLike && inv.IdempotencyKey == "" {
		return tools.Result{}, tools.ErrIdempotencyRequired
	}

	if decision.RequiresApproval {
		if inv.ApprovalID == "" {
			item, err := e.approvals.Request(ctx, approval.Approval{
				TenantID: inv.TenantID,
				UserID: inv.UserID,
				TaskID: inv.TaskID,
				AgentID: inv.AgentID,
				ToolID: inv.ToolID,
				IdempotencyKey: inv.IdempotencyKey,
				ArgumentsHash: argumentsHash,
				Risk: tool.Risk,
			})
			if err != nil { return tools.Result{}, err }
			return tools.Result{}, &tools.ApprovalRequiredError{ApprovalID: item.ID}
		}
		approved, err := e.approvals.IsApproved(ctx, inv.TenantID, inv.ApprovalID)
		if err != nil { return tools.Result{}, err }
		if !approved {
			return tools.Result{}, &tools.ApprovalRequiredError{ApprovalID: inv.ApprovalID}
		}
	}

	if writeLike {
		execution, acquired, err := e.executions.Acquire(ctx, inv, 2*time.Minute)
		if err != nil { return tools.Result{}, err }
		if !acquired {
			if execution.Status == tools.ExecutionCompleted {
				var cached tools.Result
				if err := json.Unmarshal(execution.Result, &cached); err != nil { return tools.Result{}, err }
				return cached, nil
			}
			return tools.Result{}, tools.ErrExecutionInProgress
		}
	}

	backend, ok := e.backends[tool.Backend]
	if !ok {
		return tools.Result{}, fmt.Errorf("tool backend %q unavailable", tool.Backend)
	}
	result, err := backend.Execute(ctx, tool, inv.Arguments)
	if err != nil {
		if writeLike { _ = e.executions.Fail(ctx, inv.TenantID, inv.IdempotencyKey, "backend_error") }
		_ = e.audit.Record(ctx, audit.Event{
			TenantID: inv.TenantID, UserID: inv.UserID, TaskID: inv.TaskID, AgentID: inv.AgentID,
			EventType: "tool_execution", ResourceType: "tool", ResourceID: tool.ID,
			Action: string(tool.Operation), Result: "failed", PayloadHash: argumentsHash,
		})
		return tools.Result{}, err
	}

	if writeLike {
		raw, err := json.Marshal(result)
		if err != nil { return tools.Result{}, err }
		if err := e.executions.Complete(ctx, inv.TenantID, inv.IdempotencyKey, raw); err != nil { return tools.Result{}, err }
	}
	_ = e.audit.Record(ctx, audit.Event{
		TenantID: inv.TenantID, UserID: inv.UserID, TaskID: inv.TaskID, AgentID: inv.AgentID,
		EventType: "tool_execution", ResourceType: "tool", ResourceID: tool.ID,
		Action: string(tool.Operation), Result: "success", PayloadHash: argumentsHash,
	})
	return result, nil
}

func hashArguments(raw json.RawMessage) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
