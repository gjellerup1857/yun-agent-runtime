package openai

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

const defaultBaseURL = "https://api.openai.com"

type Provider struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func New(apiKey string, client *http.Client) *Provider {
	return &Provider{apiKey: apiKey, baseURL: defaultBaseURL, http: client}
}

func (p *Provider) Name() string { return "openai" }

type responseRequest struct {
	Model           string `json:"model"`
	Instructions    string `json:"instructions,omitempty"`
	Input           string `json:"input"`
	MaxOutputTokens int    `json:"max_output_tokens,omitempty"`
	Store           bool   `json:"store"`
}

type responseEnvelope struct {
	ID    string `json:"id"`
	Model string `json:"model"`
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		} `json:"content,omitempty"`
	} `json:"output"`
	Usage struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
		TotalTokens  int64 `json:"total_tokens"`
		OutputTokensDetails struct {
			ReasoningTokens int64 `json:"reasoning_tokens"`
		} `json:"output_tokens_details"`
	} `json:"usage"`
}

func (p *Provider) Generate(ctx context.Context, req provider.Request) (provider.Response, error) {
	if strings.TrimSpace(req.Model.Model) == "" {
		return provider.Response{}, &provider.ProviderError{Provider: p.Name(), Kind: provider.ErrorBadRequest, Message: "model is required"}
	}

	payload := responseRequest{
		Model: req.Model.Model,
		Instructions: req.System,
		Input: req.Input,
		MaxOutputTokens: req.MaxOutputTokens,
		Store: false,
	}
	raw, err := json.Marshal(payload)
	if err != nil { return provider.Response{}, err }

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/responses", bytes.NewReader(raw))
	if err != nil { return provider.Response{}, err }
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := p.http.Do(httpReq)
	if err != nil {
		return provider.Response{}, &provider.ProviderError{Provider: p.Name(), Kind: provider.ErrorUnavailable, Message: err.Error(), Retryable: true, Err: err}
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(res.Body, 32*1024))
		return provider.Response{}, provider.HTTPError(p.Name(), res.StatusCode, string(message))
	}

	var envelope responseEnvelope
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		return provider.Response{}, fmt.Errorf("decode openai response: %w", err)
	}

	var text strings.Builder
	for _, item := range envelope.Output {
		if item.Type != "message" { continue }
		for _, part := range item.Content {
			if part.Type == "output_text" || part.Type == "text" {
				text.WriteString(part.Text)
			}
		}
	}

	return provider.Response{
		Text: text.String(),
		Model: envelope.Model,
		ProviderRequestID: envelope.ID,
		Usage: provider.Usage{
			InputTokens: envelope.Usage.InputTokens,
			OutputTokens: envelope.Usage.OutputTokens,
			ReasoningTokens: envelope.Usage.OutputTokensDetails.ReasoningTokens,
			TotalTokens: envelope.Usage.TotalTokens,
		},
	}, nil
}

var _ provider.Provider = (*Provider)(nil)
