# YAR Production Authentication

YAR is an OAuth-protected MCP Resource Server. It does not mint access tokens and it does not treat ChatGPT, Claude, Gemini, or any other UI platform as the canonical identity provider.

## Identity rule

The access-token subject is the canonical YAR user identifier:

```text
OAuth access token
  sub = canonical YAR users.id
        ↓
PostgreSQL users
        ↓
Tenant + User principal
        ↓
YAR Runtime / Memory / Tasks / Tools
```

YAR never accepts a canonical `user_id` from MCP tool input or a production REST request body.

## Required token properties

The configured authorization server must expose an RFC 7662-compatible introspection endpoint. Active tokens accepted by YAR must provide:

- `active: true`
- `sub`: canonical YAR user UUID
- `aud`: contains `YAR_MCP_RESOURCE_URL`
- `exp`: token expiration
- `iss`: exactly `YAR_AUTHORIZATION_SERVER` when an expected issuer is configured
- `scope`: OAuth scopes granted to the token

YAR also rejects tokens whose `nbf` is still in the future or whose `iat` is unreasonably in the future, allowing a small clock-skew window.

## Core scopes

YAR currently defines two production scopes:

| Scope | Capability |
| --- | --- |
| `yar:profile:read` | Call `yar_profile_get` |
| `yar:team:run` | Call `yar_team_run` and `POST /v1/team/run` |

The `/mcp` bearer middleware authenticates the token but does not require every scope globally. Each MCP tool enforces its own scope so a profile-only token cannot run the agent team and a team-run-only token cannot read profile data.

Additional deployment-specific scopes may be advertised through `YAR_AUTH_SUPPORTED_SCOPES`, but YAR always advertises its core scopes.

## Protected Resource Metadata

For a resource URL such as:

```text
https://api.example.com/mcp
```

YAR serves RFC 9728 Protected Resource Metadata at:

```text
https://api.example.com/.well-known/oauth-protected-resource/mcp
```

and also exposes the root metadata path for compatibility:

```text
https://api.example.com/.well-known/oauth-protected-resource
```

Unauthenticated protected requests receive a `401` response with a `WWW-Authenticate` header pointing clients to the protected-resource metadata URL.

## Environment variables

Production mode is enabled whenever `YAR_DEV_MODE` is not `true`.

```bash
YAR_DEV_MODE=false
YAR_MCP_RESOURCE_URL=https://api.example.com/mcp
YAR_AUTHORIZATION_SERVER=https://auth.example.com
YAR_AUTH_INTROSPECTION_URL=https://auth.example.com/oauth2/introspect
YAR_AUTH_CLIENT_ID=yar-resource-server
YAR_AUTH_CLIENT_SECRET=...
YAR_AUTH_SUPPORTED_SCOPES="yar:profile:read yar:team:run"
```

`YAR_AUTH_CLIENT_SECRET` is a Resource Server credential used only when calling the authorization server's introspection endpoint. It must be supplied through a secret manager in production and must never be stored in YAR memory, prompts, audit payloads, or the repository.

## Development mode

When:

```bash
YAR_DEV_MODE=true
```

YAR uses the seeded development identity and keeps the local Admin console and development tool endpoints available. Development identity headers are not trusted in production mode.

## Authorization server responsibility

The external authorization server is responsible for the browser login / consent flow, PKCE, access-token issuance, refresh-token policy, client registration or Client ID Metadata Document compatibility, revocation, and user authentication policy.

YAR remains the protected resource and canonical runtime. This keeps authentication infrastructure separate from agent/runtime authorization and avoids building a custom OAuth server inside YAR.
