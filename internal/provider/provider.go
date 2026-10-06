package provider

import "context"

type Request struct {
	Model  string
	System string
	Input  string
}

type Response struct {
	Text         string `json:"text"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
}

type Provider interface {
	Name() string
	Generate(ctx context.Context, req Request) (Response, error)
}
