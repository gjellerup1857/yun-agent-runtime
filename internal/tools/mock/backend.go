package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gjellerup1857/yun-agent-runtime/internal/tools"
)

type Backend struct {
	mu sync.Mutex
	notes []string
}

func New() *Backend { return &Backend{} }
func (b *Backend) Name() string { return "mock" }

func (b *Backend) Execute(_ context.Context, tool tools.Tool, arguments json.RawMessage) (tools.Result, error) {
	switch tool.ID {
	case "mock.note.read":
		b.mu.Lock()
		defer b.mu.Unlock()
		raw, err := json.Marshal(map[string]any{"notes": b.notes})
		if err != nil { return tools.Result{}, err }
		return tools.Result{Content: fmt.Sprintf("%d note(s)", len(b.notes)), Structured: raw}, nil

	case "mock.note.create":
		var input struct { Text string `json:"text"` }
		if err := json.Unmarshal(arguments, &input); err != nil { return tools.Result{}, err }
		b.mu.Lock()
		b.notes = append(b.notes, input.Text)
		b.mu.Unlock()
		raw, _ := json.Marshal(map[string]any{"created": true, "text": input.Text})
		return tools.Result{Content: "note created", Structured: raw}, nil
	default:
		return tools.Result{}, tools.ErrToolNotFound
	}
}

var _ tools.Backend = (*Backend)(nil)
