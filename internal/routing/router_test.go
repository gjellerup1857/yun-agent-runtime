package routing

import "testing"

func TestRouterAIBackendArchitecture(t *testing.T) {
	router := New()
	result := router.Route("請幫我設計 AI 後端 API 架構")

	expected := map[string]bool{
		"software-architect":        true,
		"technical-product-manager": true,
		"backend-engineer":          true,
		"automation-test-engineer":  true,
		"ai-engineer":               true,
	}

	for _, id := range result {
		delete(expected, id)
	}
	if len(expected) != 0 {
		t.Fatalf("missing agents: %v; got %v", expected, result)
	}
}

func TestRouterFallback(t *testing.T) {
	result := New().Route("hello")
	if len(result) != 2 || result[0] != "product-manager" || result[1] != "technical-product-manager" {
		t.Fatalf("unexpected fallback: %v", result)
	}
}
