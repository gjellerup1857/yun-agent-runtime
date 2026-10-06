package authn

import (
	"fmt"
	"net/http"
	"strings"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

const (
	mcpMethodHeader = "Mcp-Method"
	mcpNameHeader   = "Mcp-Name"
)

func MCPScopeStepUp(resourceMetadataURL string, toolScopes map[string][]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.EqualFold(strings.TrimSpace(r.Header.Get(mcpMethodHeader)), "tools/call") {
				next.ServeHTTP(w, r)
				return
			}

			name := strings.TrimSpace(r.Header.Get(mcpNameHeader))
			required := toolScopes[name]
			if name == "" || len(required) == 0 {
				// Older clients may omit the 2026-07-28 standard headers. Handler-level
				// authorization remains the final enforcement point in that case.
				next.ServeHTTP(w, r)
				return
			}

			info := mcpauth.TokenInfoFromContext(r.Context())
			if err := RequireScopes(info, required...); err == nil {
				next.ServeHTTP(w, r)
				return
			}

			params := []string{`error="insufficient_scope"`}
			params[0] = strings.ReplaceAll(params[0], `\"`, `"`)
			if len(required) > 0 {
				params = append(params, fmt.Sprintf("scope=%q", strings.Join(required, " ")))
			}
			if resourceMetadataURL != "" {
				params = append(params, fmt.Sprintf("resource_metadata=%q", resourceMetadataURL))
			}
			w.Header().Set("WWW-Authenticate", "Bearer "+strings.Join(params, ", "))
			http.Error(w, "insufficient scope", http.StatusForbidden)
		})
	}
}
