package authn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

type IntrospectionConfig struct {
	Endpoint         string
	ClientID         string
	ClientSecret     string
	ExpectedIssuer   string
	ExpectedAudience string
	ClockSkew        time.Duration
}

type IntrospectionVerifier struct {
	cfg  IntrospectionConfig
	http *http.Client
}

func NewIntrospectionVerifier(cfg IntrospectionConfig, client *http.Client) (*IntrospectionVerifier, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, fmt.Errorf("OAuth introspection endpoint is required")
	}
	if _, err := ValidateSecureURL(cfg.Endpoint); err != nil {
		return nil, fmt.Errorf("OAuth introspection endpoint is invalid: %w", err)
	}
	if strings.TrimSpace(cfg.ClientID) == "" {
		return nil, fmt.Errorf("OAuth introspection client ID is required")
	}
	if strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, fmt.Errorf("OAuth introspection client secret is required")
	}
	if strings.TrimSpace(cfg.ExpectedAudience) == "" {
		return nil, fmt.Errorf("OAuth expected audience is required")
	}
	if cfg.ClockSkew <= 0 {
		cfg.ClockSkew = 30 * time.Second
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &IntrospectionVerifier{cfg: cfg, http: client}, nil
}

type audience []string

func (a *audience) UnmarshalJSON(raw []byte) error {
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		*a = audience{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return err
	}
	*a = many
	return nil
}

type introspectionResponse struct {
	Active   bool     `json:"active"`
	Scope    string   `json:"scope"`
	ClientID string   `json:"client_id"`
	Sub      string   `json:"sub"`
	Aud      audience `json:"aud"`
	Iss      string   `json:"iss"`
	Exp      int64    `json:"exp"`
	Iat      int64    `json:"iat"`
	Nbf      int64    `json:"nbf"`
}

func (v *IntrospectionVerifier) Verify(ctx context.Context, token string, _ *http.Request) (*mcpauth.TokenInfo, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("%w: access token is empty", mcpauth.ErrInvalidToken)
	}

	form := url.Values{}
	form.Set("token", token)
	form.Set("token_type_hint", "access_token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.cfg.Endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(v.cfg.ClientID, v.cfg.ClientSecret)

	res, err := v.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OAuth introspection request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OAuth introspection returned HTTP %d", res.StatusCode)
	}

	var payload introspectionResponse
	decoder := json.NewDecoder(io.LimitReader(res.Body, 64<<10))
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode OAuth introspection response: %w", err)
	}
	if !payload.Active {
		return nil, fmt.Errorf("%w: inactive access token", mcpauth.ErrInvalidToken)
	}
	if strings.TrimSpace(payload.Sub) == "" {
		return nil, fmt.Errorf("%w: token subject is missing", mcpauth.ErrInvalidToken)
	}
	if payload.Exp <= 0 {
		return nil, fmt.Errorf("%w: token expiration is missing", mcpauth.ErrInvalidToken)
	}
	if v.cfg.ExpectedIssuer != "" && payload.Iss != v.cfg.ExpectedIssuer {
		return nil, fmt.Errorf("%w: issuer mismatch", mcpauth.ErrInvalidToken)
	}
	if !containsAudience(payload.Aud, v.cfg.ExpectedAudience) {
		return nil, fmt.Errorf("%w: audience mismatch", mcpauth.ErrInvalidToken)
	}

	now := time.Now()
	expiresAt := time.Unix(payload.Exp, 0)
	if now.Add(-v.cfg.ClockSkew).After(expiresAt) {
		return nil, fmt.Errorf("%w: access token expired", mcpauth.ErrInvalidToken)
	}
	if payload.Nbf > 0 && now.Add(v.cfg.ClockSkew).Before(time.Unix(payload.Nbf, 0)) {
		return nil, fmt.Errorf("%w: access token is not active yet", mcpauth.ErrInvalidToken)
	}
	if payload.Iat > 0 && now.Add(v.cfg.ClockSkew).Before(time.Unix(payload.Iat, 0)) {
		return nil, fmt.Errorf("%w: access token issued-at is in the future", mcpauth.ErrInvalidToken)
	}

	return &mcpauth.TokenInfo{
		Scopes:     strings.Fields(payload.Scope),
		Expiration: expiresAt,
		UserID:     payload.Sub,
		Extra: map[string]any{
			"issuer":     payload.Iss,
			"audience":   []string(payload.Aud),
			"client_id":  payload.ClientID,
			"issued_at":  payload.Iat,
			"not_before": payload.Nbf,
		},
	}, nil
}

func containsAudience(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
