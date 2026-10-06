package platformgateway

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/gjellerup1857/yun-agent-runtime/internal/platformstate"
	yarruntime "github.com/gjellerup1857/yun-agent-runtime/internal/runtime"
)

type fakeState struct {
	conversation platformstate.Conversation
	receipt      platformstate.Receipt
	acquired     bool
	completed    json.RawMessage
	attachedTask string
}

func (f *fakeState) ResolveOrCreateConversation(context.Context, string, string, string, string, string, string) (platformstate.Conversation, error) {
	return f.conversation, nil
}

func (f *fakeState) AttachTask(_ context.Context, _ string, taskID string) error {
	f.attachedTask = taskID
	return nil
}

func (f *fakeState) AcquireReceipt(context.Context, string, string, string, string, time.Duration) (platformstate.AcquireResult, error) {
	if len(f.completed) > 0 {
		return platformstate.AcquireResult{Receipt: platformstate.Receipt{Status: platformstate.ReceiptCompleted, Response: f.completed}}, nil
	}
	return platformstate.AcquireResult{Receipt: f.receipt, Acquired: f.acquired}, nil
}

func (f *fakeState) CompleteReceipt(_ context.Context, _ string, response json.RawMessage) error {
	f.completed = append(json.RawMessage(nil), response...)
	return nil
}

func (f *fakeState) FailReceipt(context.Context, string, string) error { return nil }

type fakeRunner struct {
	calls int
	resp  yarruntime.RunResponse
	err   error
}

func (f *fakeRunner) Run(context.Context, yarruntime.RunRequest) (yarruntime.RunResponse, error) {
	f.calls++
	return f.resp, f.err
}

func TestReplayReturnsCachedResponseWithoutRerunningRuntime(t *testing.T) {
	state := &fakeState{
		conversation: platformstate.Conversation{ID: "conversation-1"},
		receipt:      platformstate.Receipt{ID: "receipt-1", Status: platformstate.ReceiptProcessing},
		acquired:     true,
	}
	runner := &fakeRunner{resp: yarruntime.RunResponse{RunID: "run-1", TaskID: "task-1", Answer: "done"}}
	service := New(state, runner)
	req := Request{
		TenantID: "tenant-1", UserID: "user-1", SourceClient: "chatgpt",
		ExternalConversationID: "conversation-ext", ExternalMessageID: "message-ext", Message: "continue",
	}

	first, err := service.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if first.Cached || runner.calls != 1 || state.attachedTask != "task-1" {
		t.Fatalf("unexpected first run: cached=%v calls=%d attached=%q", first.Cached, runner.calls, state.attachedTask)
	}

	state.acquired = false
	second, err := service.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Cached {
		t.Fatal("expected replay to return cached response")
	}
	if runner.calls != 1 {
		t.Fatalf("runtime reran on replay: calls=%d", runner.calls)
	}
	if second.Run.TaskID != "task-1" || second.Run.Answer != "done" {
		t.Fatalf("unexpected cached response: %+v", second)
	}
}

func TestActiveReceiptReturnsInProgress(t *testing.T) {
	state := &fakeState{
		conversation: platformstate.Conversation{ID: "conversation-1"},
		receipt:      platformstate.Receipt{ID: "receipt-1", Status: platformstate.ReceiptProcessing},
		acquired:     false,
	}
	runner := &fakeRunner{}
	service := New(state, runner)
	_, err := service.Run(context.Background(), Request{
		TenantID: "tenant-1", UserID: "user-1", SourceClient: "claude",
		ExternalConversationID: "conversation-ext", ExternalMessageID: "message-ext", Message: "continue",
	})
	if !errors.Is(err, ErrMessageInProgress) {
		t.Fatalf("err=%v, want ErrMessageInProgress", err)
	}
	if runner.calls != 0 {
		t.Fatalf("runtime should not run while receipt lease is active: calls=%d", runner.calls)
	}
}
