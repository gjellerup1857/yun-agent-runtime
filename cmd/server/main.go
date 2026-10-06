package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/gjellerup1857/yun-agent-runtime/internal/adminapi"
	"github.com/gjellerup1857/yun-agent-runtime/internal/approval"
	"github.com/gjellerup1857/yun-agent-runtime/internal/audit"
	"github.com/gjellerup1857/yun-agent-runtime/internal/authn"
	"github.com/gjellerup1857/yun-agent-runtime/internal/identity"
	"github.com/gjellerup1857/yun-agent-runtime/internal/inference"
	"github.com/gjellerup1857/yun-agent-runtime/internal/mcpserver"
	"github.com/gjellerup1857/yun-agent-runtime/internal/memory"
	"github.com/gjellerup1857/yun-agent-runtime/internal/platformapi"
	"github.com/gjellerup1857/yun-agent-runtime/internal/platformgateway"
	"github.com/gjellerup1857/yun-agent-runtime/internal/platformstate"
	"github.com/gjellerup1857/yun-agent-runtime/internal/policy"
	"github.com/gjellerup1857/yun-agent-runtime/internal/provider"
	anthropicprovider "github.com/gjellerup1857/yun-agent-runtime/internal/provider/anthropic"
	googleprovider "github.com/gjellerup1857/yun-agent-runtime/internal/provider/google"
	openaiprovider "github.com/gjellerup1857/yun-agent-runtime/internal/provider/openai"
	"github.com/gjellerup1857/yun-agent-runtime/internal/routing"
	yarruntime "github.com/gjellerup1857/yun-agent-runtime/internal/runtime"
	"github.com/gjellerup1857/yun-agent-runtime/internal/task"
	"github.com/gjellerup1857/yun-agent-runtime/internal/toolruntime"
	"github.com/gjellerup1857/yun-agent-runtime/internal/tools"
	mocktools "github.com/gjellerup1857/yun-agent-runtime/internal/tools/mock"
)

const (
	devTenantID = "00000000-0000-0000-0000-000000000001"
	devUserID   = "00000000-0000-0000-0000-000000000101"
)

type oauthConfig struct {
	ResourceURL         string
	AuthorizationServer string
	IntrospectionURL    string
	ClientID            string
	ClientSecret        string
	SupportedScopes     []string
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	devMode := os.Getenv("YAR_DEV_MODE") == "true"
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://yar:yar_dev@localhost:5432/yar"
	}
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := db.Ping(pingCtx); err != nil {
		log.Fatalf("database unavailable: %v", err)
	}

	memoryRepo := memory.NewPostgresRepository(db)
	taskRepo := task.NewPostgresRepository(db)
	identityResolver := identity.NewPostgresResolver(db)
	agentRouter := routing.New()

	providerRegistry := provider.NewRegistry()
	providerRegistry.Register(provider.NewMock())
	models := map[string]provider.ModelRef{
		"mock": {ID: "mock", Provider: "mock", Model: "mock-balanced"},
	}
	httpClient := provider.NewHTTPClient()

	if key, model := os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_MODEL"); key != "" && model != "" {
		providerRegistry.Register(openaiprovider.New(key, httpClient))
		models["openai"] = provider.ModelRef{ID: "openai", Provider: "openai", Model: model}
	}
	if key, model := os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("ANTHROPIC_MODEL"); key != "" && model != "" {
		providerRegistry.Register(anthropicprovider.New(key, httpClient))
		models["anthropic"] = provider.ModelRef{ID: "anthropic", Provider: "anthropic", Model: model}
	}
	if key, model := os.Getenv("GEMINI_API_KEY"), os.Getenv("GEMINI_MODEL"); key != "" && model != "" {
		providerRegistry.Register(googleprovider.New(key, httpClient))
		models["google"] = provider.ModelRef{ID: "google", Provider: "google", Model: model}
	}

	modelRouter := routing.NewModelRouter(models)
	inferenceService := inference.New(providerRegistry)
	runtime := yarruntime.NewMulti(
		agentRouter,
		modelRouter,
		inferenceService,
		memoryRepo,
		taskRepo,
		memory.NewExtractor(),
	)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(tools.Tool{
		ID:          "mock.note.read",
		Description: "Read mock development notes.",
		Operation:   tools.OperationRead,
		Risk:        tools.RiskLow,
		Backend:     "mock",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	})
	toolRegistry.Register(tools.Tool{
		ID:          "mock.note.create",
		Description: "Create a mock development note.",
		Operation:   tools.OperationWrite,
		Risk:        tools.RiskMedium,
		Backend:     "mock",
		InputSchema: json.RawMessage(`{"type":"object","required":["text"]}`),
	})

	policyEngine := policy.New(map[string]policy.AgentPolicy{
		"product-manager": {
			Allow:            map[string]struct{}{"mock.note.read": {}},
			ApprovalRequired: map[string]struct{}{"mock.note.create": {}},
			Deny:             map[string]struct{}{},
		},
		"technical-product-manager": {
			Allow:            map[string]struct{}{"mock.note.read": {}},
			ApprovalRequired: map[string]struct{}{"mock.note.create": {}},
			Deny:             map[string]struct{}{},
		},
		"backend-engineer": {
			Allow:            map[string]struct{}{"mock.note.read": {}},
			ApprovalRequired: map[string]struct{}{"mock.note.create": {}},
			Deny:             map[string]struct{}{},
		},
		"software-architect": {
			Allow:            map[string]struct{}{"mock.note.read": {}},
			ApprovalRequired: map[string]struct{}{},
			Deny:             map[string]struct{}{"mock.note.create": {}},
		},
	})

	approvalRepo := approval.NewPostgresRepository(db)
	approvalService := approval.NewService(approvalRepo)
	executionRepo := tools.NewPostgresExecutionRepository(db)
	auditRepo := audit.NewPostgresRepository(db)
	toolExecutor := toolruntime.NewExecutor(
		toolRegistry,
		tools.NewBasicSchemaValidator(),
		policyEngine,
		approvalService,
		executionRepo,
		auditRepo,
		[]tools.Backend{mocktools.New()},
	)
	runtime.WithTools(toolRegistry, toolExecutor, policyEngine)

	platformStore := platformstate.NewStore(db)
	platformGateway := platformgateway.New(platformStore, runtime)

	mux := http.NewServeMux()
	var mcpBearerMiddleware func(http.Handler) http.Handler
	var mcpScopeStepUp func(http.Handler) http.Handler
	var teamRunBearerMiddleware func(http.Handler) http.Handler
	authMode := "development"
	metadataURL := ""

	if devMode {
		adminapi.New(db, devTenantID, devUserID).Register(mux)
	} else {
		cfg, err := loadOAuthConfig()
		if err != nil {
			log.Fatalf("OAuth configuration invalid: %v", err)
		}
		verifier, err := authn.NewIntrospectionVerifier(authn.IntrospectionConfig{
			Endpoint:         cfg.IntrospectionURL,
			ClientID:         cfg.ClientID,
			ClientSecret:     cfg.ClientSecret,
			ExpectedIssuer:   cfg.AuthorizationServer,
			ExpectedAudience: cfg.ResourceURL,
			ClockSkew:        30 * time.Second,
		}, &http.Client{Timeout: 10 * time.Second})
		if err != nil {
			log.Fatalf("OAuth verifier configuration invalid: %v", err)
		}

		metadataURL, metadataPaths, err := protectedResourceMetadataLocations(cfg.ResourceURL)
		if err != nil {
			log.Fatalf("OAuth protected resource metadata invalid: %v", err)
		}
		metadataHandler := mcpauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
			Resource:               cfg.ResourceURL,
			AuthorizationServers:   []string{cfg.AuthorizationServer},
			ScopesSupported:        cfg.SupportedScopes,
			BearerMethodsSupported: []string{"header"},
			ResourceName:            "Yun Agent Runtime MCP",
		})
		for _, path := range metadataPaths {
			mux.Handle(path, metadataHandler)
		}

		canonicalVerifier := authn.RequireCanonicalUser(verifier.Verify, identityResolver)
		mcpBearerMiddleware = mcpauth.RequireBearerToken(canonicalVerifier, &mcpauth.RequireBearerTokenOptions{
			ResourceMetadataURL: metadataURL,
			ClockSkew:           30 * time.Second,
		})
		mcpScopeStepUp = authn.MCPScopeStepUp(metadataURL, map[string][]string{
			"yar_profile_get": {authn.ScopeProfileRead},
			"yar_team_run":    {authn.ScopeTeamRun},
		})
		teamRunBearerMiddleware = mcpauth.RequireBearerToken(canonicalVerifier, &mcpauth.RequireBearerTokenOptions{
			ResourceMetadataURL: metadataURL,
			Scopes:              []string{authn.ScopeTeamRun},
			ClockSkew:           30 * time.Second,
		})
		authMode = "oauth-introspection"
	}

	devTenant, devUser := "", ""
	if devMode {
		devTenant, devUser = devTenantID, devUserID
	}
	mcpEndpoint := mcpserver.New(runtime, identityResolver, devTenant, devUser)
	mcpHandler := mcpEndpoint.Handler()
	if mcpScopeStepUp != nil {
		mcpHandler = mcpScopeStepUp(mcpHandler)
	}
	if mcpBearerMiddleware != nil {
		mcpHandler = mcpBearerMiddleware(mcpHandler)
	}
	mux.Handle("/mcp", mcpHandler)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		checkCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(checkCtx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "degraded", "database": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":                "ok",
			"database":              "ok",
			"mcp":                   "/mcp",
			"auth_mode":             authMode,
			"mcp_protected":         !devMode,
			"resource_metadata_url": metadataURL,
			"providers": map[string]bool{
				"mock":      providerRegistry.Has("mock"),
				"openai":    providerRegistry.Has("openai"),
				"anthropic": providerRegistry.Has("anthropic"),
				"google":    providerRegistry.Has("google"),
			},
		})
	})

	principalResolver := func(r *http.Request) (identity.Principal, error) {
		return resolveHTTPPrincipal(r, devMode, identityResolver)
	}

	teamRunHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req yarruntime.RunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
			return
		}

		principal, err := principalResolver(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authenticated identity required"})
			return
		}
		req.TenantID = principal.TenantID
		req.UserID = principal.UserID

		result, err := runtime.Run(r.Context(), req)
		if err != nil {
			log.Printf("run failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	if teamRunBearerMiddleware != nil {
		mux.Handle("POST /v1/team/run", teamRunBearerMiddleware(teamRunHandler))
	} else {
		mux.Handle("POST /v1/team/run", teamRunHandler)
	}

	platformRunHandler := platformapi.New(platformGateway, principalResolver)
	if teamRunBearerMiddleware != nil {
		mux.Handle("POST /v1/platform/run", teamRunBearerMiddleware(platformRunHandler))
	} else {
		mux.Handle("POST /v1/platform/run", platformRunHandler)
	}

	if devMode {
		mux.HandleFunc("POST /v1/dev/tools/execute", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				AgentID        string          `json:"agent_id"`
				ToolID         string          `json:"tool_id"`
				TaskID         string          `json:"task_id"`
				IdempotencyKey string          `json:"idempotency_key"`
				ApprovalID     string          `json:"approval_id"`
				Arguments      json.RawMessage `json:"arguments"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
				return
			}
			result, err := toolExecutor.Execute(r.Context(), tools.Invocation{
				TenantID:       devTenantID,
				UserID:         devUserID,
				TaskID:         body.TaskID,
				AgentID:        body.AgentID,
				ToolID:         body.ToolID,
				Arguments:      body.Arguments,
				IdempotencyKey: body.IdempotencyKey,
				ApprovalID:     body.ApprovalID,
			})
			if err != nil {
				var approvalErr *tools.ApprovalRequiredError
				if errors.As(err, &approvalErr) {
					writeJSON(w, http.StatusConflict, map[string]any{
						"error":       "APPROVAL_REQUIRED",
						"approval_id": approvalErr.ApprovalID,
					})
					return
				}
				if errors.Is(err, tools.ErrToolDenied) {
					writeJSON(w, http.StatusForbidden, map[string]any{"error": "TOOL_DENIED"})
					return
				}
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, result)
		})

		mux.HandleFunc("POST /v1/dev/approvals/{id}/approve", func(w http.ResponseWriter, r *http.Request) {
			approvalID := r.PathValue("id")
			if err := approvalRepo.Resolve(r.Context(), devTenantID, devUserID, approvalID, approval.StatusApproved); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"approved": true, "approval_id": approvalID})
		})
	}

	addr := os.Getenv("YAR_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("YAR listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown failed: %v", err)
	}
}

func resolveHTTPPrincipal(r *http.Request, devMode bool, resolver identity.Resolver) (identity.Principal, error) {
	if devMode {
		tenantID := strings.TrimSpace(r.Header.Get("X-YAR-Tenant-ID"))
		userID := strings.TrimSpace(r.Header.Get("X-YAR-User-ID"))
		if tenantID == "" {
			tenantID = devTenantID
		}
		if userID == "" {
			userID = devUserID
		}
		return identity.Principal{TenantID: tenantID, UserID: userID, DisplayName: "YAR Developer"}, nil
	}

	info := mcpauth.TokenInfoFromContext(r.Context())
	if info == nil || strings.TrimSpace(info.UserID) == "" {
		return identity.Principal{}, fmt.Errorf("bearer token identity missing")
	}
	return resolver.Resolve(r.Context(), info.UserID)
}

func loadOAuthConfig() (oauthConfig, error) {
	configuredScopes := parseScopes(os.Getenv("YAR_AUTH_SUPPORTED_SCOPES"))
	if len(configuredScopes) == 0 {
		configuredScopes = parseScopes(os.Getenv("YAR_AUTH_REQUIRED_SCOPES"))
	}
	cfg := oauthConfig{
		ResourceURL:         strings.TrimSpace(os.Getenv("YAR_MCP_RESOURCE_URL")),
		AuthorizationServer: strings.TrimRight(strings.TrimSpace(os.Getenv("YAR_AUTHORIZATION_SERVER")), "/"),
		IntrospectionURL:    strings.TrimSpace(os.Getenv("YAR_AUTH_INTROSPECTION_URL")),
		ClientID:            strings.TrimSpace(os.Getenv("YAR_AUTH_CLIENT_ID")),
		ClientSecret:        os.Getenv("YAR_AUTH_CLIENT_SECRET"),
		SupportedScopes: mergeScopes(
			[]string{authn.ScopeProfileRead, authn.ScopeTeamRun},
			configuredScopes,
		),
	}

	missing := make([]string, 0, 5)
	for name, value := range map[string]string{
		"YAR_MCP_RESOURCE_URL":       cfg.ResourceURL,
		"YAR_AUTHORIZATION_SERVER":   cfg.AuthorizationServer,
		"YAR_AUTH_INTROSPECTION_URL": cfg.IntrospectionURL,
		"YAR_AUTH_CLIENT_ID":         cfg.ClientID,
		"YAR_AUTH_CLIENT_SECRET":     cfg.ClientSecret,
	} {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return oauthConfig{}, fmt.Errorf("missing environment variables: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

func parseScopes(raw string) []string {
	raw = strings.ReplaceAll(raw, ",", " ")
	return strings.Fields(raw)
}

func mergeScopes(groups ...[]string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, group := range groups {
		for _, scope := range group {
			scope = strings.TrimSpace(scope)
			if scope == "" {
				continue
			}
			if _, ok := seen[scope]; ok {
				continue
			}
			seen[scope] = struct{}{}
			out = append(out, scope)
		}
	}
	return out
}

func protectedResourceMetadataLocations(resource string) (string, []string, error) {
	u, err := url.Parse(resource)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", nil, fmt.Errorf("resource must be an absolute URL")
	}
	if u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", nil, fmt.Errorf("resource URL must not contain user info, query, or fragment")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && isLoopbackHost(u.Hostname())) {
		return "", nil, fmt.Errorf("resource URL must use HTTPS unless it is loopback")
	}

	const root = "/.well-known/oauth-protected-resource"
	resourcePath := strings.TrimSuffix(u.Path, "/")
	metadataPath := root
	if resourcePath != "" {
		metadataPath += resourcePath
	}
	metadataURL := (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: metadataPath}).String()
	paths := []string{metadataPath}
	if metadataPath != root {
		paths = append(paths, root)
	}
	return metadataURL, paths, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
