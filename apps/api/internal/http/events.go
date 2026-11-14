package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gabedsam01/gabe-whats/shared/contracts"
)

// registerEventsTicket publica o emissor de tickets do SSE.
func (s *Server) registerEventsTicket() {
	type output struct {
		Body *contracts.SSETicketResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/events/ticket",
		OperationID: "createEventsTicket",
		Summary:     "Emite ticket efêmero para abrir o stream SSE",
		Tags:        []string{"Webhooks & Events"},
	}, func(_ context.Context, _ *struct{}) (*output, error) {
		if s.deps.Tickets == nil {
			return nil, newProblem(http.StatusServiceUnavailable, "sse_unavailable", "stream SSE não inicializado")
		}
		ticket, expires := s.deps.Tickets.Issue()
		return &output{Body: &contracts.SSETicketResponse{
			Ticket:    ticket,
			ExpiresAt: expires,
			StreamURL: fmt.Sprintf("http://%s/api/v1/events/stream?ticket=%s", s.cfg.HTTPAddr, ticket),
		}}, nil
	})
}

// registerStream registra o stream SSE como rota chi (resposta contínua não
// cabe no modelo request/response do Huma).
func (s *Server) registerStream() {
	s.router.Get("/api/v1/events/stream", s.handleStream)
}

// handleStream mantém a conexão aberta entregando eventos e heartbeats.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if s.deps.Hub == nil {
		writeProblem(w, http.StatusServiceUnavailable, "sse_unavailable", "stream SSE não inicializado")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeProblem(w, http.StatusInternalServerError, "streaming_unsupported", "o servidor não suporta streaming")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, ": conectado\n\n")
	flusher.Flush()

	stream, cancel := s.deps.Hub.Subscribe(64)
	defer cancel()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case payload, open := <-stream:
			if !open {
				return
			}
			if _, err := fmt.Fprintf(w, "event: message\ndata: %s\n\n", payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
