package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gjellerup1857/yun-agent-runtime/internal/provider"
	"github.com/gjellerup1857/yun-agent-runtime/internal/toolruntime"
	"github.com/gjellerup1857/yun-agent-runtime/internal/tools"
)

type toolLoop struct {
	executor *toolruntime.Executor
}

type waitingApprovalError struct {
	ApprovalID string
}

func (e *waitingApprovalError) Error() string {
	return fmt.Sprintf("waiting for approval %s", e.ApprovalID)
}

func newToolLoop(executor *toolruntime.Executor) *toolLoop {
	return &toolLoop{executor: executor}
}

func (l *toolLoop) executeCalls(
	ctx context.Context,
	req RunRequest,
	agentID string,
	calls []provider.ToolCall,
) ([]provider.ToolResult, error) {
	results := make([]provider.ToolResult, 0, len(calls))
	for index, call := range calls {
		idempotencyKey := fmt.Sprintf("%s:%s:%s:%d", req.TaskID, agentID, call.ID, index)
		result, err := l.executor.Execute(ctx, tools.Invocation{
			TenantID:       req.TenantID,
			UserID:         req.UserID,
			TaskID:         req.TaskID,
			AgentID:        agentID,
			ToolID:         call.Name,
			Arguments:      json.RawMessage(call.Arguments),
			IdempotencyKey: idempotencyKey,
		})
		if err != nil {
			var approvalErr *tools.ApprovalRequiredError
			if errors.As(err, &approvalErr) {
				return nil, &waitingApprovalError{ApprovalID: approvalErr.ApprovalID}
			}
			results = append(results, provider.ToolResult{
				ToolCallID: call.ID,
				Content:    err.Error(),
				IsError:    true,
			})
			continue
		}

		content := result.Content
		if len(result.Structured) > 0 {
			content += "\n" + string(result.Structured)
		}
		results = append(results, provider.ToolResult{
			ToolCallID: call.ID,
			Content:    content,
		})
	}
	return results, nil
}
