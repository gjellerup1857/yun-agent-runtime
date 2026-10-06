package provider

import (
	"context"
	"fmt"
)

type Mock struct{}

func NewMock() *Mock { return &Mock{} }

func (m *Mock) Name() string { return "mock" }

func (m *Mock) Generate(_ context.Context, req Request) (Response, error) {
	model := req.Model
	if model == "" {
		model = "mock-balanced"
	}
	text := fmt.Sprintf("[mock:%s] 已收到任務：%s", model, req.Input)
	in := int64(len([]rune(req.System+req.Input))/2 + 1)
	out := int64(len([]rune(text))/2 + 1)
	return Response{Text: text, InputTokens: in, OutputTokens: out}, nil
}

var _ Provider = (*Mock)(nil)
