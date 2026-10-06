package memory

import "strings"

type ExtractRequest struct {
	TenantID string
	UserID string
	TeamID string
	ProjectID string
	TaskID string
	Message string
}

type Extractor struct{}

func NewExtractor() *Extractor { return &Extractor{} }

func (e *Extractor) Extract(req ExtractRequest) []Memory {
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return nil
	}
	if m, ok := extractBackendLanguageDecision(req); ok {
		return []Memory{m}
	}
	return nil
}

func extractBackendLanguageDecision(req ExtractRequest) (Memory, bool) {
	lower := strings.ToLower(req.Message)
	if !(strings.Contains(lower, "backend") || strings.Contains(lower, "後端")) {
		return Memory{}, false
	}
	if !(strings.Contains(lower, "預設") || strings.Contains(lower, "改成") || strings.Contains(lower, "default") || strings.Contains(lower, "使用")) {
		return Memory{}, false
	}

	languages := []struct{ keyword, name string }{
		{"rust", "Rust"},
		{"python", "Python"},
		{"mojo", "Mojo"},
		{"golang", "Go"},
		{" go ", "Go"},
	}
	padded := " " + lower + " "
	for _, language := range languages {
		if strings.Contains(padded, language.keyword) {
			scope := "user"
			if req.ProjectID != "" {
				scope = "project"
			}
			return Memory{
				TenantID: req.TenantID,
				UserID: req.UserID,
				TeamID: req.TeamID,
				ProjectID: req.ProjectID,
				TaskID: req.TaskID,
				Scope: scope,
				Type: TypeDecision,
				Key: "backend.default_language",
				Content: "Backend 預設使用 " + language.name,
				Importance: 0.98,
				Confidence: 0.98,
				TrustScore: 1,
				SourceType: "user",
			}, true
		}
	}
	return Memory{}, false
}
