// Command server inicializa o daemon da API REST: carrega a configuração,
// abre a sessão do WhatsApp, sobe o servidor HTTP com documentação e trata o
// encerramento gracioso.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gabedsam01/gabe-whats/apps/api/internal/config"
	httpapi "github.com/gabedsam01/gabe-whats/apps/api/internal/http"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/whatsapp"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	if err := run(); err != nil {
		slog.Error("encerrando com erro", "erro", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	setupLogger(cfg.LogLevel)
	slog.Info("iniciando gabe-whats", "versao", httpapi.Version, "endereco", cfg.HTTPAddr)

	if !cfg.IsLoopback() {
		slog.Warn("a API está escutando em endereço não local", "endereco", cfg.HTTPAddr)
	}

	wa, err := whatsapp.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer wa.Close()

	server := httpapi.New(cfg, wa)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	listenErr := make(chan error, 1)
	go func() {
		slog.Info("API disponível", "docs", "http://"+cfg.HTTPAddr+"/docs", "openapi", "http://"+cfg.HTTPAddr+"/openapi.yaml")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErr <- err
		}
	}()

	select {
	case err := <-listenErr:
		return err
	case <-ctx.Done():
		slog.Info("sinal recebido; encerrando graciosamente")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Warn("encerramento do servidor HTTP incompleto", "erro", err)
	}
	return nil
}

// setupLogger configura o slog com o nível vindo da configuração.
func setupLogger(level string) {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	slog.SetDefault(slog.New(handler))
}
