package anthropic

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

const defaultBaseURL = "https://api.anthropic.com"

type Provider struct {
	apiKey string
	baseURL string
	http *http.Client
}

func New(apiKey string, client *http.Client) *Provider {
	return &Provider{apiKey: apiKey, baseURL: defaultBaseURL, http: client}
}

func (p *Provider) Name() string { return "anthropic" }

type message struct {
	Role string `json:"role"`
	Content string `json:"content"`
}

type requestBody struct {
	Model string `json:"model"`
	System string `json:"system,omitempty"`
	MaxTokens int `json:"max_tokens"`
	Messages []message `json:"messages"`
	Stream bool `json:"stream"`
}

type responseBody struct {
	ID string `json:"id"`
	Model string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"content"`
	Usage struct {
		InputTokens int64 `json:"input_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		CacheReadInputTokens int64 `json:"cache_read_input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
		OutputTokensDetails struct {
			ThinkingTokens int64 `json:"thinking_tokens"`
		} `json:"output_tokens_details"`
	} `json:"usage"`
}

func (p *Provider) Generate(ctx context.Context, req provider.Request) (provider.Response, error) {
	maxTokens := req.MaxOutputTokens
	if maxTokens <= 0 { maxTokens = 4096 }
	payload := requestBody{
		Model: req.Model.Model,
		System: req.System,
		MaxTokens: maxTokens,
		Messages: []message{{Role: "user", Content: req.Input}},
		Stream: false,
	}
	raw, err := json.Marshal(payload)
	if err != nil { return provider.Response{}, err }

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(raw))
	if err != nil { return provider.Response{}, err }
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

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
		return provider.Response{}, fmt.Errorf("decode anthropic response: %w", err)
	}
	var text strings.Builder
	for _, block := range envelope.Content {
		if block.Type == "text" { text.WriteString(block.Text) }
	}
	inputTokens := envelope.Usage.InputTokens + envelope.Usage.CacheCreationInputTokens + envelope.Usage.CacheReadInputTokens
	outputTokens := envelope.Usage.OutputTokens
	return provider.Response{
		Text: text.String(),
		Model: envelope.Model,
		ProviderRequestID: envelope.ID,
		Usage: provider.Usage{
			InputTokens: inputTokens,
			OutputTokens: outputTokens,
			ReasoningTokens: envelope.Usage.OutputTokensDetails.ThinkingTokens,
			TotalTokens: inputTokens + outputTokens,
		},
	}, nil
}

var _ provider.Provider = (*Provider)(nil)
