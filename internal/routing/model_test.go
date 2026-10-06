package routing

import (
	"testing"

	"github.com/gjellerup1857/yun-agent-runtime/internal/provider"
)

func TestBackendPrefersAnthropicCandidate(t *testing.T) {
	router := NewModelRouter(map[string]provider.ModelRef{
		"openai":    {Provider: "openai", Model: "o"},
		"anthropic": {Provider: "anthropic", Model: "a"},
		"google":    {Provider: "google", Model: "g"},
		"mock":      {Provider: "mock", Model: "m"},
	})
	result := router.Route("backend-engineer")
	if len(result) != 4 || result[0].Provider != "anthropic" {
		t.Fatalf("unexpected backend model order: %+v", result)
	}
}

func TestArchitectPrefersOpenAICandidate(t *testing.T) {
	router := NewModelRouter(map[string]provider.ModelRef{
		"openai":    {Provider: "openai", Model: "o"},
		"anthropic": {Provider: "anthropic", Model: "a"},
		"mock":      {Provider: "mock", Model: "m"},
	})
	result := router.Route("software-architect")
	if len(result) == 0 || result[0].Provider != "openai" {
		t.Fatalf("unexpected architect model order: %+v", result)
	}
}

func TestRouterSkipsMissingProviders(t *testing.T) {
	router := NewModelRouter(map[string]provider.ModelRef{
		"mock": {Provider: "mock", Model: "mock-balanced"},
	})
	result := router.Route("backend-engineer")
	if len(result) != 1 || result[0].Provider != "mock" {
		t.Fatalf("expected mock-only route, got %+v", result)
	}
}
