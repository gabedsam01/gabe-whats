// Package sse implementa o hub de Server-Sent Events e os tickets efêmeros que
// autenticam conexões de navegador sem expor a chave da API na URL.
package sse

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// Hub distribui eventos para todos os assinantes conectados.
type Hub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
	log     *slog.Logger
}

// NewHub cria um hub vazio.
func NewHub(logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	return &Hub{clients: make(map[chan []byte]struct{}), log: logger}
}

// Subscribe registra um assinante e devolve o canal e a função de cancelamento.
// O canal é fechado pelo cancelamento; eventos são descartados se o buffer
// estiver cheio, para nunca bloquear o publisher.
func (h *Hub) Subscribe(buffer int) (<-chan []byte, func()) {
	if buffer < 1 {
		buffer = 1
	}
	ch := make(chan []byte, buffer)

	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.clients, ch)
			close(ch)
			h.mu.Unlock()
		})
	}
	return ch, cancel
}

// Broadcast serializa e envia o valor a todos os assinantes.
func (h *Hub) Broadcast(value any) {
	payload, err := json.Marshal(value)
	if err != nil {
		h.log.Warn("sse: não foi possível serializar o evento", "erro", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- payload:
		default:
			h.log.Warn("sse: assinante lento; evento descartado")
		}
	}
}

// Count devolve o número de assinantes conectados.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Tickets emite e valida tickets de uso único para o SSE.
//
// O motivo: a API do navegador (EventSource) não permite enviar headers, e
// colocar a chave da API na URL a exporia em logs e histórico. O fluxo é
// autenticar o POST com header, receber um ticket de curta duração e trocá-lo
// pela conexão.
type Tickets struct {
	mu  sync.Mutex
	ttl time.Duration
	m   map[string]time.Time
}

// NewTickets cria o emissor com o TTL informado.
func NewTickets(ttl time.Duration) *Tickets {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &Tickets{ttl: ttl, m: make(map[string]time.Time)}
}

// Issue cria um ticket novo, de uso único.
func (t *Tickets) Issue() (string, time.Time) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// rand.Read só falha em cenários catastróficos; sem ticket não há SSE.
		panic("sse: falha ao gerar ticket: " + err.Error())
	}
	id := hex.EncodeToString(raw[:])
	expires := time.Now().Add(t.ttl)

	t.mu.Lock()
	t.m[id] = expires
	t.mu.Unlock()

	return id, expires
}

// Redeem valida e consome o ticket. Um ticket só pode ser usado uma vez.
func (t *Tickets) Redeem(id string) bool {
	if id == "" {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	expires, ok := t.m[id]
	if !ok {
		return false
	}
	delete(t.m, id)

	return time.Now().Before(expires)
}

// Cleanup remove tickets vencidos.
func (t *Tickets) Cleanup() {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, expires := range t.m {
		if now.After(expires) {
			delete(t.m, id)
		}
	}
}
