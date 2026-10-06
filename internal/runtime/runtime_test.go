package runtime

import (
	"context"
	"testing"

	"github.com/gjellerup1857/yun-agent-runtime/internal/provider"
	"github.com/gjellerup1857/yun-agent-runtime/internal/routing"
)

func TestRun(t *testing.T) {
	r := New(routing.New(), provider.NewMock())
	resp, err := r.Run(context.Background(), RunRequest{Message: "請設計 Go 後端 API"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.AgentsUsed) == 0 {
		t.Fatal("expected agents")
	}
	if resp.Answer == "" {
		t.Fatal("expected answer")
	}
}

func TestRunRejectsEmptyMessage(t *testing.T) {
	r := New(routing.New(), provider.NewMock())
	if _, err := r.Run(context.Background(), RunRequest{Message: "   "}); err == nil {
		t.Fatal("expected validation error")
	}
}
