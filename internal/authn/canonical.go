package authn

import (
	"context"
	"fmt"
	"net/http"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/gjellerup1857/yun-agent-runtime/internal/identity"
)

func RequireCanonicalUser(base mcpauth.TokenVerifier, resolver identity.Resolver) mcpauth.TokenVerifier {
	return func(ctx context.Context, token string, req *http.Request) (*mcpauth.TokenInfo, error) {
		info, err := base(ctx, token, req)
		if err != nil {
			return nil, err
		}
		if info == nil || info.UserID == "" {
			return nil, fmt.Errorf("%w: canonical user subject missing", mcpauth.ErrInvalidToken)
		}
		principal, err := resolver.Resolve(ctx, info.UserID)
		if err != nil {
			return nil, fmt.Errorf("%w: canonical user unavailable", mcpauth.ErrInvalidToken)
		}
		if info.Extra == nil {
			info.Extra = map[string]any{}
		}
		info.Extra["yar_tenant_id"] = principal.TenantID
		info.Extra["yar_display_name"] = principal.DisplayName
		return info, nil
	}
}
