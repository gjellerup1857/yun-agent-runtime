package authn

import (
	"fmt"
	"strings"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

const (
	ScopeProfileRead = "yar:profile:read"
	ScopeTeamRun     = "yar:team:run"
)

func RequireScopes(info *mcpauth.TokenInfo, required ...string) error {
	if info == nil {
		return fmt.Errorf("%w: bearer token context missing", mcpauth.ErrInvalidToken)
	}

	granted := make(map[string]struct{}, len(info.Scopes))
	for _, scope := range info.Scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" {
			granted[scope] = struct{}{}
		}
	}

	for _, scope := range required {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, ok := granted[scope]; !ok {
			return fmt.Errorf("insufficient scope: %s", scope)
		}
	}
	return nil
}
