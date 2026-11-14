// Command mcp-server expõe o gabe-whats como servidor MCP (Model Context
// Protocol) para agentes como OpenClaw, Hermes, Moltbot e Agent Zero.
//
// O binário é stateless: toda operação é uma chamada HTTP à API REST
// configurada em GABE_WHATS_API_URL, autenticada com GABE_WHATS_API_KEY.
package main

import (
	"context"
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
	transport := flag.String("transport", "stdio", "transporte MCP: stdio ou sse")
	sseAddr := flag.String("sse-addr", "127.0.0.1:8090", "endereço HTTP quando --transport=sse")
	flag.Parse()

	if err := run(*transport, *sseAddr); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func run(transport, sseAddr string) error {
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

	switch strings.ToLower(strings.TrimSpace(transport)) {
	case "stdio":
		slog.Info("servidor MCP em stdio", "api", baseURL, "versao", Version)
		return newServer().Run(ctx, &mcp.StdioTransport{})

	case "sse":
		handler := mcp.NewSSEHandler(func(*http.Request) *mcp.Server { return newServer() }, nil)
		httpServer := &http.Server{
			Addr:              sseAddr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		}

		listenErr := make(chan error, 1)
		go func() {
			slog.Info("servidor MCP em SSE", "endereco", sseAddr, "api", baseURL, "versao", Version)
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

	default:
		return fmt.Errorf("transporte inválido %q: use stdio ou sse", transport)
	}
}

// envOr devolve a variável de ambiente ou o padrão.
func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
