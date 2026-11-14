// Command server inicializa o daemon da API REST: carrega a configuração,
// abre a sessão do WhatsApp e os bancos, sobe o servidor HTTP com documentação,
// dispara webhooks/SSE e trata o encerramento gracioso.
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

	"github.com/gabedsam01/gabe-whats/apps/api/internal/appstore"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/config"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/events"
	httpapi "github.com/gabedsam01/gabe-whats/apps/api/internal/http"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/media"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/sse"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/webhook"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/whatsapp"

	_ "github.com/mattn/go-sqlite3"
)

// ticketTTL é a validade do ticket de SSE.
const ticketTTL = 60 * time.Second

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

	app, err := appstore.Open(ctx, cfg.AppDSN)
	if err != nil {
		return err
	}
	defer func() { _ = app.Close() }()

	mediaStore, err := media.New(app, cfg.MediaDir, cfg.BaseURL(), cfg.MediaURLTTL, nil)
	if err != nil {
		return err
	}

	hub := sse.NewHub(nil)
	tickets := sse.NewTickets(ticketTTL)

	dispatcher, err := webhook.New(webhook.DefaultPolicy, nil, nil)
	if err != nil {
		return err
	}

	bus := events.NewBus(events.NewRecent(200), hub, dispatcher, func(ctx context.Context) (*appstore.WebhookSettings, error) {
		settings, err := app.GetWebhook(ctx)
		if err == nil {
			return settings, nil
		}
		if !errors.Is(err, appstore.ErrNotFound) {
			return nil, err
		}
		return &appstore.WebhookSettings{
			URL:               cfg.Webhook.URL,
			Secret:            cfg.Webhook.Secret,
			IgnoreFromMe:      cfg.Webhook.IgnoreFromMe,
			AllowJIDs:         cfg.Webhook.AllowJIDs,
			AutoDownloadMedia: cfg.Webhook.AutoDownloadMedia,
			IncludeBase64:     cfg.Webhook.IncludeBase64,
		}, nil
	}, nil)

	wa, err := whatsapp.New(ctx, cfg, bus, mediaStore)
	if err != nil {
		return err
	}
	defer wa.Close()

	server := httpapi.New(cfg, httpapi.Deps{
		WhatsApp: wa,
		Bus:      bus,
		Hub:      hub,
		Tickets:  tickets,
		App:      app,
		Media:    mediaStore,
	})

	go maintenance(ctx, mediaStore, tickets)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	listenErr := make(chan error, 1)
	go func() {
		slog.Info("API disponível", "docs", cfg.BaseURL()+"/docs", "openapi", cfg.BaseURL()+"/openapi.yaml")
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

// maintenance limpa mídias vencidas e tickets antigos periodicamente.
func maintenance(ctx context.Context, mediaStore *media.Store, tickets *sse.Tickets) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mediaStore.Purge(ctx)
			tickets.Cleanup()
		}
	}
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
