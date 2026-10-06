package authn

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

func TestIntrospectionVerifierSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Basic Y2xpZW50OnNlY3JldA=="; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"active": true,
			"scope": "yar:profile:read yar:team:run",
			"client_id": "mcp-client",
			"sub": "00000000-0000-0000-0000-000000000101",
			"aud": ["https://api.yar.example/mcp"],
			"iss": "https://auth.yar.example",
			"exp": 4102444800
		}`))
	}))
	defer server.Close()

	verifier, err := NewIntrospectionVerifier(IntrospectionConfig{
		Endpoint:         server.URL,
		ClientID:         "client",
		ClientSecret:     "secret",
		ExpectedIssuer:   "https://auth.yar.example",
		ExpectedAudience: "https://api.yar.example/mcp",
	}, server.Client())
	if err != nil {
		t.Fatal(err)
	}

	info, err := verifier.Verify(context.Background(), "token", nil)
	if err != nil {
		t.Fatal(err)
	}
	if info.UserID != "00000000-0000-0000-0000-000000000101" {
		t.Fatalf("UserID = %q", info.UserID)
	}
	if info.Expiration.Before(time.Now()) {
		t.Fatalf("expiration is not in the future: %v", info.Expiration)
	}
}

func TestIntrospectionVerifierRejectsInactiveToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"active":false}`))
	}))
	defer server.Close()

	verifier, err := NewIntrospectionVerifier(IntrospectionConfig{
		Endpoint:         server.URL,
		ClientID:         "client",
		ClientSecret:     "secret",
		ExpectedAudience: "https://api.yar.example/mcp",
	}, server.Client())
	if err != nil {
		t.Fatal(err)
	}

	_, err = verifier.Verify(context.Background(), "token", nil)
	if !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func TestIntrospectionVerifierRejectsWrongAudience(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"active": true,
			"sub": "00000000-0000-0000-0000-000000000101",
			"aud": "https://other.example/mcp",
			"exp": 4102444800
		}`))
	}))
	defer server.Close()

	verifier, err := NewIntrospectionVerifier(IntrospectionConfig{
		Endpoint:         server.URL,
		ClientID:         "client",
		ClientSecret:     "secret",
		ExpectedAudience: "https://api.yar.example/mcp",
	}, server.Client())
	if err != nil {
		t.Fatal(err)
	}

	_, err = verifier.Verify(context.Background(), "token", nil)
	if !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func TestIntrospectionVerifierRejectsMissingIssuerWhenIssuerExpected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"active": true,
			"sub": "00000000-0000-0000-0000-000000000101",
			"aud": "https://api.yar.example/mcp",
			"exp": 4102444800
		}`))
	}))
	defer server.Close()

	verifier, err := NewIntrospectionVerifier(IntrospectionConfig{
		Endpoint:         server.URL,
		ClientID:         "client",
		ClientSecret:     "secret",
		ExpectedIssuer:   "https://auth.yar.example",
		ExpectedAudience: "https://api.yar.example/mcp",
	}, server.Client())
	if err != nil {
		t.Fatal(err)
	}

	_, err = verifier.Verify(context.Background(), "token", nil)
	if !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func TestIntrospectionVerifierRejectsFutureNotBefore(t *testing.T) {
	nbf := time.Now().Add(5 * time.Minute).Unix()
	exp := time.Now().Add(time.Hour).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{
			"active": true,
			"sub": "00000000-0000-0000-0000-000000000101",
			"aud": "https://api.yar.example/mcp",
			"exp": %d,
			"nbf": %d
		}`, exp, nbf)
	}))
	defer server.Close()

	verifier, err := NewIntrospectionVerifier(IntrospectionConfig{
		Endpoint:         server.URL,
		ClientID:         "client",
		ClientSecret:     "secret",
		ExpectedAudience: "https://api.yar.example/mcp",
		ClockSkew:        5 * time.Second,
	}, server.Client())
	if err != nil {
		t.Fatal(err)
	}

	_, err = verifier.Verify(context.Background(), "token", nil)
	if !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}
