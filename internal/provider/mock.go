package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type Mock struct{}

func NewMock() *Mock { return &Mock{} }

func (m *Mock) Name() string { return "mock" }

func (m *Mock) Generate(_ context.Context, req Request) (Response, error) {
	model := req.Model.Model
	if model == "" {
		model = "mock-balanced"
	}

	lower := strings.ToLower(req.Input)
	if (strings.Contains(lower, "讀取測試筆記") || strings.Contains(lower, "read test notes")) && len(req.ToolResults) == 0 {
		for _, tool := range req.Tools {
			if tool.Name == "mock.note.read" {
				return Response{
					Model:             model,
					ProviderRequestID: "mock-tool-call",
					ToolCalls: []ToolCall{{
						ID:        "mock-call-read-notes",
						Name:      "mock.note.read",
						Arguments: json.RawMessage(`{}`),
					}},
				}, nil
			}
		}
	}

	if len(req.ToolResults) > 0 {
		var b strings.Builder
		b.WriteString("[mock:")
		b.WriteString(model)
		b.WriteString("] 工具執行完成：")
		for i, result := range req.ToolResults {
			if i > 0 { b.WriteString(" | ") }
			b.WriteString(result.Content)
		}
		text := b.String()
		out := int64(len([]rune(text))/2 + 1)
		return Response{Text: text, Model: model, ProviderRequestID: "mock-after-tools", Usage: Usage{OutputTokens: out, TotalTokens: out}}, nil
	}

	text := fmt.Sprintf("[mock:%s] 已收到任務：%s", model, req.Input)
	inputTokens := int64(len([]rune(req.System+req.Input))/2 + 1)
	outputTokens := int64(len([]rune(text))/2 + 1)
	return Response{
		Text:              text,
		Model:             model,
		ProviderRequestID: "mock",
		Usage: Usage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  inputTokens + outputTokens,
		},
	}, nil
}

var _ Provider = (*Mock)(nil)
