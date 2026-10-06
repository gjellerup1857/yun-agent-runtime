package provider

import "context"

type ModelRef struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type Request struct {
	Model           ModelRef
	System          string
	Input           string
	MaxOutputTokens int
	Reasoning       string
}

type Usage struct {
	InputTokens     int64 `json:"input_tokens"`
	OutputTokens    int64 `json:"output_tokens"`
	ReasoningTokens int64 `json:"reasoning_tokens"`
	TotalTokens     int64 `json:"total_tokens"`
}

type Response struct {
	Text              string `json:"text"`
	Model             string `json:"model,omitempty"`
	ProviderRequestID string `json:"provider_request_id,omitempty"`
	Usage             Usage  `json:"usage"`
}

type Provider interface {
	Name() string
	Generate(ctx context.Context, req Request) (Response, error)
}
