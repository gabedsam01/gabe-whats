// Command mcp-server expõe o gabe-whats como servidor MCP (Model Context
// Protocol) para agentes como OpenClaw, Hermes, Moltbot, Agent Zero e agentes.chat.
//
// O binário é stateless: toda operação é uma chamada HTTP à API REST
// configurada em GABE_WHATS_API_URL, autenticada com GABE_WHATS_API_KEY.
package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gabedsam01/gabe-whats/apps/mcp/internal/apiclient"
	"github.com/gabedsam01/gabe-whats/apps/mcp/internal/resources"
	"github.com/gabedsam01/gabe-whats/apps/mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version é a versão do servidor MCP.
const Version = "0.5.0-s5"

func main() {
	defaultTransport := envOr("GABE_WHATS_MCP_TRANSPORT", "stdio")
	defaultAddr := envOr("GABE_WHATS_MCP_ADDR", "127.0.0.1:8090")

	transport := flag.String("transport", defaultTransport, "transporte MCP: stdio, http (Streamable HTTP) ou sse")
	httpAddr := flag.String("http-addr", "", "endereço HTTP quando --transport=http (padrão: valor de --sse-addr)")
	sseAddr := flag.String("sse-addr", defaultAddr, "endereço HTTP quando --transport=sse ou fallback de --http-addr")
	flag.Parse()

	if err := run(*transport, *httpAddr, *sseAddr); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func run(transport, httpAddr, sseAddr string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	baseURL := envOr("GABE_WHATS_API_URL", "http://127.0.0.1:8080")
	apiKey := strings.TrimSpace(os.Getenv("GABE_WHATS_API_KEY"))
	if apiKey == "" {
		return errors.New("GABE_WHATS_API_KEY não definida: o servidor MCP precisa da chave da API REST")
	}

	client := apiclient.New(baseURL, apiKey)
	newServer := func() *mcp.Server {
		server := mcp.NewServer(&mcp.Implementation{
			Name:        "gabe-whats",
			Title:       "gabe-whats",
			Description: "Gateway pessoal de WhatsApp: mensagens, mídias, grupos e eventos.",
			Version:     Version,
		}, nil)
		tools.Register(server, client)
		resources.Register(server, client)
		return server
	}

	mode := strings.ToLower(strings.TrimSpace(transport))
	switch mode {
	case "stdio":
		slog.Info("servidor MCP em stdio", "api", baseURL, "versao", Version)
		return newServer().Run(ctx, &mcp.StdioTransport{})

	case "http", "sse":
		addr := strings.TrimSpace(sseAddr)
		if mode == "http" && strings.TrimSpace(httpAddr) != "" {
			addr = strings.TrimSpace(httpAddr)
		}
		if addr == "" {
			return errors.New("endereço HTTP vazio para o servidor MCP")
		}

		// Listener HTTP/SSE nunca sobe sem token: usa GABE_WHATS_MCP_TOKEN quando
		// configurada ou reutiliza GABE_WHATS_API_KEY como fallback seguro.
		authToken := envOr("GABE_WHATS_MCP_TOKEN", apiKey)
		handler, err := newHTTPHandler(mode, authToken, newServer)
		if err != nil {
			return err
		}

		return serveHTTP(ctx, mode, addr, baseURL, handler)

	default:
		return fmt.Errorf("transporte inválido %q: use stdio, http ou sse", transport)
	}
}

// newHTTPHandler monta o roteador HTTP do servidor MCP para os transportes
// "http" (Streamable HTTP) ou "sse", exigindo Bearer token nas rotas MCP e
// mantendo GET /health público para healthcheck de containers e proxy reverso.
func newHTTPHandler(transport, authToken string, newServer func() *mcp.Server) (http.Handler, error) {
	mode := strings.ToLower(strings.TrimSpace(transport))
	if strings.TrimSpace(authToken) == "" {
		return nil, errors.New("token de autenticação HTTP do MCP não pode ser vazio")
	}
	if newServer == nil {
		return nil, errors.New("fábrica do servidor MCP não informada")
	}

	var mcpHandler http.Handler
	switch mode {
	case "http":
		// DisableLocalhostProtection permite atender atrás de proxy reverso (ex.: Caddy)
		// com cabeçalho Host público; a proteção do endpoint é garantida pelo Bearer token.
		mcpHandler = mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return newServer()
		}, &mcp.StreamableHTTPOptions{
			DisableLocalhostProtection: true,
		})
	case "sse":
		mcpHandler = mcp.NewSSEHandler(func(*http.Request) *mcp.Server {
			return newServer()
		}, nil)
	default:
		return nil, fmt.Errorf("transporte HTTP inválido %q: use http ou sse", transport)
	}

	protected := requireBearerAuth(authToken, mcpHandler)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":    "ok",
			"version":   Version,
			"transport": mode,
		})
	})
	mux.Handle("/mcp", protected)
	mux.Handle("/mcp/", protected)
	mux.Handle("/sse", protected)
	mux.Handle("/", protected)
	return mux, nil
}

// serveHTTP inicia o servidor HTTP do MCP e trata desligamento gracioso.
func serveHTTP(ctx context.Context, mode, addr, baseURL string, handler http.Handler) error {
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	listenErr := make(chan error, 1)
	go func() {
		slog.Info("servidor MCP HTTP ativo", "transporte", mode, "endereco", addr, "api", baseURL, "versao", Version)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErr <- err
		}
	}()

	select {
	case err := <-listenErr:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

// requireBearerAuth exige Authorization: Bearer <token> ou X-API-Key com
// comparação em tempo constante.
func requireBearerAuth(token string, next http.Handler) http.Handler {
	expected := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := extractAuthToken(r)
		if subtle.ConstantTimeCompare([]byte(got), expected) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":  "unauthorized",
				"detail": "token Bearer ausente ou inválido",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// extractAuthToken lê o token do header Authorization (Bearer case-insensitive)
// ou do header X-API-Key.
func extractAuthToken(r *http.Request) string {
	if auth := strings.TrimSpace(r.Header.Get("Authorization")); auth != "" {
		scheme, rest, ok := strings.Cut(auth, " ")
		if ok && strings.EqualFold(scheme, "Bearer") {
			return strings.TrimSpace(rest)
		}
	}
	return strings.TrimSpace(r.Header.Get("X-API-Key"))
}

// envOr devolve a variável de ambiente ou o padrão.
func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
