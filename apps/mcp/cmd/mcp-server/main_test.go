package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gabedsam01/gabe-whats/apps/mcp/internal/apiclient"
	"github.com/gabedsam01/gabe-whats/apps/mcp/internal/resources"
	"github.com/gabedsam01/gabe-whats/apps/mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newTestMCPServerFactory() func() *mcp.Server {
	client := apiclient.New("http://127.0.0.1:9", "chave-api-teste-123456")
	return func() *mcp.Server {
		server := mcp.NewServer(&mcp.Implementation{
			Name:    "gabe-whats-teste",
			Version: Version,
		}, nil)
		tools.Register(server, client)
		resources.Register(server, client)
		return server
	}
}

func TestStreamableHTTPHealthEAuth(t *testing.T) {
	const token = "segredo-mcp-streamable-teste"

	handler, err := newHTTPHandler("http", token, newTestMCPServerFactory())
	if err != nil {
		t.Fatalf("newHTTPHandler: %v", err)
	}

	// 1. GET /health é público (para healthcheck de container e proxy).
	healthReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthRec := httptest.NewRecorder()
	handler.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d; esperado 200", healthRec.Code)
	}
	var healthBody map[string]string
	if err := json.Unmarshal(healthRec.Body.Bytes(), &healthBody); err != nil {
		t.Fatalf("JSON inválido em /health: %v", err)
	}
	if healthBody["status"] != "ok" || healthBody["transport"] != "http" {
		t.Fatalf("corpo inesperado em /health: %#v", healthBody)
	}

	// 2. POST /mcp sem token ou com token inválido deve retornar 401 + WWW-Authenticate.
	cases := []struct {
		name   string
		header string
	}{
		{name: "sem authorization", header: ""},
		{name: "bearer inválido", header: "Bearer token-errado"},
		{name: "esquema inválido", header: "Basic dXNlcjpwYXNz"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d; esperado 401 (body=%s)", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("WWW-Authenticate = %q; esperado Bearer", got)
			}
		})
	}
}

func TestStreamableHTTPListToolsAtrasDeProxy(t *testing.T) {
	const token = "segredo-mcp-streamable-teste"

	handler, err := newHTTPHandler("http", token, newTestMCPServerFactory())
	if err != nil {
		t.Fatalf("newHTTPHandler: %v", err)
	}

	srv := httptest.NewServer(handler)
	defer srv.Close()

	baseTransport := srv.Client().Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}

	httpClient := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			clone := r.Clone(r.Context())
			clone.Header.Set("Authorization", "Bearer "+token)
			// Simula Host público encaminhado pelo Caddy em proxy reverso.
			clone.Host = "whats.exemplo.com.br"
			return baseTransport.RoundTrip(clone)
		}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "teste-streamable-cliente", Version: "0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   srv.URL + "/mcp",
		HTTPClient: httpClient,
	}, nil)
	if err != nil {
		t.Fatalf("conectar via Streamable HTTP: %v", err)
	}
	defer func() { _ = session.Close() }()

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("listar tools via Streamable HTTP: %v", err)
	}
	if len(result.Tools) != 22 {
		t.Fatalf("esperava 22 tools, obtive %d", len(result.Tools))
	}
}

func TestNewHTTPHandlerRejeitaTokenVazio(t *testing.T) {
	if _, err := newHTTPHandler("http", "   ", newTestMCPServerFactory()); err == nil {
		t.Fatal("esperava erro ao criar handler HTTP sem token")
	}
}
