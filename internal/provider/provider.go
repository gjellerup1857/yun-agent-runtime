package provider

import (
	"context"
	"encoding/json"
)

type ModelRef struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error"`
}

type Request struct {
	Model           ModelRef
	System          string
	Input           string
	MaxOutputTokens int
	Reasoning       string
	Tools           []ToolDefinition
	ToolResults     []ToolResult
}

type Usage struct {
	InputTokens     int64 `json:"input_tokens"`
	OutputTokens    int64 `json:"output_tokens"`
	ReasoningTokens int64 `json:"reasoning_tokens"`
	TotalTokens     int64 `json:"total_tokens"`
}

type Response struct {
	Text              string     `json:"text"`
	Model             string     `json:"model,omitempty"`
	ProviderRequestID string     `json:"provider_request_id,omitempty"`
	ToolCalls         []ToolCall `json:"tool_calls,omitempty"`
	Usage             Usage      `json:"usage"`
}

type Provider interface {
	Name() string
	Generate(ctx context.Context, req Request) (Response, error)
}
