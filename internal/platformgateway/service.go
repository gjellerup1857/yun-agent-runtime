package platformgateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gjellerup1857/yun-agent-runtime/internal/platformstate"
	yarruntime "github.com/gjellerup1857/yun-agent-runtime/internal/runtime"
)

var ErrMessageInProgress = errors.New("platform message is already processing")

type StateStore interface {
	ResolveOrCreateConversation(ctx context.Context, tenantID, userID, sourceClient, externalConversationID, teamID, projectID string) (platformstate.Conversation, error)
	AttachTask(ctx context.Context, conversationID, taskID string) error
	AcquireReceipt(ctx context.Context, tenantID, userID, sourceClient, externalMessageID string, lease time.Duration) (platformstate.AcquireResult, error)
	CompleteReceipt(ctx context.Context, receiptID string, response json.RawMessage) error
	FailReceipt(ctx context.Context, receiptID, errorCode string) error
}

type Runner interface {
	Run(ctx context.Context, req yarruntime.RunRequest) (yarruntime.RunResponse, error)
}

type Service struct {
	state StateStore
	runner Runner
	lease time.Duration
}

type Request struct {
	TenantID               string `json:"-"`
	UserID                 string `json:"-"`
	SourceClient           string `json:"source_client"`
	ExternalConversationID string `json:"external_conversation_id"`
	ExternalMessageID      string `json:"external_message_id"`
	ProjectID              string `json:"project_id,omitempty"`
	TaskID                 string `json:"task_id,omitempty"`
	Message                string `json:"message"`
}

type Response struct {
	ConversationID string                 `json:"conversation_id"`
	Cached         bool                   `json:"cached"`
	Run            yarruntime.RunResponse `json:"run"`
}

func New(state StateStore, runner Runner) *Service {
	return &Service{state: state, runner: runner, lease: 2 * time.Minute}
}

func (s *Service) Run(ctx context.Context, req Request) (Response, error) {
	if s.state == nil || s.runner == nil {
		return Response{}, fmt.Errorf("platform gateway dependencies are unavailable")
	}
	if strings.TrimSpace(req.TenantID) == "" || strings.TrimSpace(req.UserID) == "" {
		return Response{}, fmt.Errorf("canonical tenant and user identity are required")
	}
	if strings.TrimSpace(req.SourceClient) == "" || strings.TrimSpace(req.ExternalConversationID) == "" || strings.TrimSpace(req.ExternalMessageID) == "" {
		return Response{}, fmt.Errorf("source client, conversation ID, and message ID are required")
	}
	if strings.TrimSpace(req.Message) == "" {
		return Response{}, fmt.Errorf("message is required")
	}

	conversation, err := s.state.ResolveOrCreateConversation(
		ctx,
		req.TenantID,
		req.UserID,
		req.SourceClient,
		req.ExternalConversationID,
		"yun-product-engineering",
		req.ProjectID,
	)
	if err != nil {
		return Response{}, err
	}

	acquired, err := s.state.AcquireReceipt(
		ctx,
		req.TenantID,
		req.UserID,
		req.SourceClient,
		req.ExternalMessageID,
		s.lease,
	)
	if err != nil {
		return Response{}, err
	}
	if !acquired.Acquired {
		if acquired.Receipt.Status == platformstate.ReceiptCompleted && len(acquired.Receipt.Response) > 0 {
			var cached Response
			if err := json.Unmarshal(acquired.Receipt.Response, &cached); err != nil {
				return Response{}, fmt.Errorf("decode cached platform response: %w", err)
			}
			cached.Cached = true
			return cached, nil
		}
		return Response{}, ErrMessageInProgress
	}

	result, err := s.runner.Run(ctx, yarruntime.RunRequest{
		TenantID:  req.TenantID,
		UserID:    req.UserID,
		ProjectID: req.ProjectID,
		TaskID:    req.TaskID,
		Message:   req.Message,
	})
	if err != nil {
		_ = s.state.FailReceipt(ctx, acquired.Receipt.ID, "runtime_error")
		return Response{}, err
	}
	if err := s.state.AttachTask(ctx, conversation.ID, result.TaskID); err != nil {
		_ = s.state.FailReceipt(ctx, acquired.Receipt.ID, "attach_task_error")
		return Response{}, err
	}

	response := Response{
		ConversationID: conversation.ID,
		Cached:         false,
		Run:            result,
	}
	raw, err := json.Marshal(response)
	if err != nil {
		_ = s.state.FailReceipt(ctx, acquired.Receipt.ID, "encode_response_error")
		return Response{}, err
	}
	if err := s.state.CompleteReceipt(ctx, acquired.Receipt.ID, raw); err != nil {
		return Response{}, err
	}
	return response, nil
}
