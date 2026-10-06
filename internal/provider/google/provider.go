package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gjellerup1857/yun-agent-runtime/internal/provider"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com"

type Provider struct {
	apiKey string
	baseURL string
	http *http.Client
}

func New(apiKey string, client *http.Client) *Provider {
	return &Provider{apiKey: apiKey, baseURL: defaultBaseURL, http: client}
}

func (p *Provider) Name() string { return "google" }

type generationConfig struct {
	MaxOutputTokens int `json:"max_output_tokens,omitempty"`
}

type requestBody struct {
	Model string `json:"model"`
	Input string `json:"input"`
	SystemInstruction string `json:"system_instruction,omitempty"`
	Store bool `json:"store"`
	GenerationConfig *generationConfig `json:"generation_config,omitempty"`
}

type responseBody struct {
	ID string `json:"id"`
	Model string `json:"model"`
	Status string `json:"status"`
	Steps []struct {
		Type string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		} `json:"content,omitempty"`
	} `json:"steps"`
	Usage struct {
		TotalInputTokens int64 `json:"total_input_tokens"`
		TotalOutputTokens int64 `json:"total_output_tokens"`
		TotalThoughtTokens int64 `json:"total_thought_tokens"`
		TotalTokens int64 `json:"total_tokens"`
	} `json:"usage"`
}

func (p *Provider) Generate(ctx context.Context, req provider.Request) (provider.Response, error) {
	payload := requestBody{
		Model: req.Model.Model,
		Input: req.Input,
		SystemInstruction: req.System,
		Store: false,
	}
	if req.MaxOutputTokens > 0 {
		payload.GenerationConfig = &generationConfig{MaxOutputTokens: req.MaxOutputTokens}
	}
	raw, err := json.Marshal(payload)
	if err != nil { return provider.Response{}, err }

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/interactions", bytes.NewReader(raw))
	if err != nil { return provider.Response{}, err }
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.apiKey)

	res, err := p.http.Do(httpReq)
	if err != nil {
		return provider.Response{}, &provider.ProviderError{Provider: p.Name(), Kind: provider.ErrorUnavailable, Message: err.Error(), Retryable: true, Err: err}
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(res.Body, 32*1024))
		return provider.Response{}, provider.HTTPError(p.Name(), res.StatusCode, string(message))
	}

	var envelope responseBody
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		return provider.Response{}, fmt.Errorf("decode gemini interaction: %w", err)
	}
	var text strings.Builder
	for _, step := range envelope.Steps {
		if step.Type != "model_output" { continue }
		for _, part := range step.Content {
			if part.Type == "text" { text.WriteString(part.Text) }
		}
	}
	return provider.Response{
		Text: text.String(),
		Model: envelope.Model,
		ProviderRequestID: envelope.ID,
		Usage: provider.Usage{
			InputTokens: envelope.Usage.TotalInputTokens,
			OutputTokens: envelope.Usage.TotalOutputTokens,
			ReasoningTokens: envelope.Usage.TotalThoughtTokens,
			TotalTokens: envelope.Usage.TotalTokens,
		},
	}, nil
}

var _ provider.Provider = (*Provider)(nil)
