package routing

import "strings"

type Router struct{}

func New() *Router { return &Router{} }

func (r *Router) Route(message string) []string {
	input := strings.ToLower(message)
	result := make([]string, 0, 9)
	add := func(id string) {
		for _, existing := range result {
			if existing == id {
				return
			}
		}
		result = append(result, id)
	}

	if contains(input, "prd", "產品", "需求", "roadmap", "ux", "wireframe", "prototype", "kyc", "aml", "seo") {
		add("product-manager")
		add("technical-product-manager")
	}
	if contains(input, "架構", "architecture", "distributed", "微服務", "microservice", "system design") {
		add("software-architect")
		add("technical-product-manager")
	}
	if contains(input, "frontend", "前端", "preact", "react", "typescript", "html", "css", "swift", "kotlin") {
		add("frontend-engineer")
		add("automation-test-engineer")
	}
	if contains(input, "backend", "後端", "api", "go", "golang", "mojo", "database", "資料庫") {
		add("backend-engineer")
		add("automation-test-engineer")
	}
	if contains(input, "fullstack", "全端", "cloud", "雲端", "成本", "docker") {
		add("fullstack-engineer")
	}
	if contains(input, "ai", "llm", "rag", "agent", "機器學習", "machine learning", "embedding", "token") {
		add("ai-engineer")
		add("software-architect")
	}
	if contains(input, "test", "測試", "qa", "bug", "回歸") {
		add("automation-test-engineer")
	}
	if len(result) == 0 {
		add("product-manager")
		add("technical-product-manager")
	}
	return result
}

func contains(input string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(input, strings.ToLower(term)) {
			return true
		}
	}
	return false
}
