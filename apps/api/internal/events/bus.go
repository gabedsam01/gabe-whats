// Package events centraliza a distribuição de eventos normalizados: histórico
// recente, stream SSE e entrega por webhook.
package events

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gabedsam01/gabe-whats/apps/api/internal/appstore"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/sse"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/webhook"
	"github.com/gabedsam01/gabe-whats/shared/contracts"
)

// SettingsProvider devolve a configuração de webhook vigente.
type SettingsProvider func(ctx context.Context) (*appstore.WebhookSettings, error)

// Bus distribui eventos para os consumidores registrados.
type Bus struct {
	recent     *Recent
	hub        *sse.Hub
	dispatcher *webhook.Dispatcher
	settings   SettingsProvider
	log        *slog.Logger
}

// NewBus monta o barramento.
func NewBus(recent *Recent, hub *sse.Hub, dispatcher *webhook.Dispatcher, settings SettingsProvider, logger *slog.Logger) *Bus {
	if logger == nil {
		logger = slog.Default()
	}
	return &Bus{recent: recent, hub: hub, dispatcher: dispatcher, settings: settings, log: logger}
}

// Recent devolve o histórico em memória.
func (b *Bus) Recent() *Recent { return b.recent }

// Publish entrega o evento ao histórico, ao SSE e (assincronamente) ao webhook.
func (b *Bus) Publish(ctx context.Context, event contracts.WebhookInboundEvent) {
	if b.recent != nil {
		b.recent.Add(event)
	}
	if b.hub != nil {
		b.hub.Broadcast(event)
	}
	if b.dispatcher == nil || b.settings == nil {
		return
	}

	go b.deliver(event)
}

// deliver aplica os filtros e dispara a entrega com a política de reentrega.
func (b *Bus) deliver(event contracts.WebhookInboundEvent) {
	// A entrega continua mesmo se a requisição de origem terminar, mas tem um
	// teto próprio para não vazar goroutine indefinidamente.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	settings, err := b.settings(ctx)
	if err != nil || settings == nil || settings.URL == "" {
		return
	}

	if !shouldDeliver(settings, event) {
		return
	}

	cfg := contracts.WebhookConfig{
		URL:               settings.URL,
		Secret:            settings.Secret,
		IgnoreFromMe:      settings.IgnoreFromMe,
		AllowJIDs:         settings.AllowJIDs,
		AutoDownloadMedia: settings.AutoDownloadMedia,
		IncludeBase64:     settings.IncludeBase64,
	}

	result := b.dispatcher.Deliver(ctx, cfg, event)
	if !result.Succeeded() {
		b.log.Error("webhook: entrega esgotada",
			"event_id", event.EventID,
			"tentativas", result.Attempts,
			"status", result.StatusCode,
			"erro", result.Err,
		)
	}
}

// shouldDeliver aplica IgnoreFromMe e AllowJIDs.
func shouldDeliver(settings *appstore.WebhookSettings, event contracts.WebhookInboundEvent) bool {
	if event.EventType != contracts.EventMessageReceived {
		return true
	}
	if settings.IgnoreFromMe && event.IsFromMe {
		return false
	}
	if len(settings.AllowJIDs) == 0 {
		return true
	}
	for _, allowed := range settings.AllowJIDs {
		if matchesJID(allowed, event.ChatJID) || matchesJID(allowed, event.SenderJID) {
			return true
		}
	}
	return false
}

// matchesJID compara ignorando caixa e espaços.
func matchesJID(allowed, actual string) bool {
	return strings.EqualFold(strings.TrimSpace(allowed), strings.TrimSpace(actual))
}

// Recent guarda as últimas mensagens recebidas em memória.
type Recent struct {
	mu   sync.RWMutex
	max  int
	data []contracts.WebhookInboundEvent
}

// NewRecent cria o histórico com capacidade máxima.
func NewRecent(max int) *Recent {
	if max < 1 {
		max = 100
	}
	return &Recent{max: max, data: make([]contracts.WebhookInboundEvent, 0, max)}
}

// Add insere o evento (apenas mensagens) no histórico.
func (r *Recent) Add(event contracts.WebhookInboundEvent) {
	if event.EventType != contracts.EventMessageReceived {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.data = append(r.data, event)
	if len(r.data) > r.max {
		r.data = r.data[len(r.data)-r.max:]
	}
}

// List devolve as mensagens mais recentes, da mais nova para a mais antiga.
// chatJID vazio devolve todos os chats.
func (r *Recent) List(chatJID string, limit int) []contracts.WebhookInboundEvent {
	if limit < 1 {
		limit = 50
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]contracts.WebhookInboundEvent, 0, limit)
	for i := len(r.data) - 1; i >= 0 && len(out) < limit; i-- {
		if chatJID != "" && !strings.EqualFold(r.data[i].ChatJID, chatJID) {
			continue
		}
		out = append(out, r.data[i])
	}
	return out
}
