package provider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMockProviderToolCallHandshake(t *testing.T) {
	mock := NewMock()

	first, err := mock.Generate(context.Background(), Request{
		Model: ModelRef{Provider: "mock", Model: "mock-balanced"},
		Input: "請讀取測試筆記",
		Tools: []ToolDefinition{
			{
				Name:        "mock.note.read",
				Description: "Read notes",
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.ToolCalls) != 1 || first.ToolCalls[0].Name != "mock.note.read" {
		t.Fatalf("expected mock.note.read tool call, got %+v", first.ToolCalls)
	}

	second, err := mock.Generate(context.Background(), Request{
		Model: ModelRef{Provider: "mock", Model: "mock-balanced"},
		Input: "請讀取測試筆記",
		Tools: []ToolDefinition{{Name: "mock.note.read"}},
		ToolResults: []ToolResult{
			{ToolCallID: first.ToolCalls[0].ID, Content: "0 note(s)"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.ToolCalls) != 0 {
		t.Fatalf("expected final response without tool calls, got %+v", second.ToolCalls)
	}
	if !strings.Contains(second.Text, "0 note(s)") {
		t.Fatalf("expected tool result in final text, got %q", second.Text)
	}
}
