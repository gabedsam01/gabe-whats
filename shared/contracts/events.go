package contracts

import "time"

// SSETicketResponse é o ticket efêmero de uso único usado para abrir o stream
// SSE. A chave da API vai no header do POST; a URL do stream carrega apenas o
// ticket, que expira e é consumido na primeira conexão.
type SSETicketResponse struct {
	Ticket    string    `json:"ticket" doc:"Ticket de uso único; apresente em GET /api/v1/events/stream?ticket=<ticket>"`
	ExpiresAt time.Time `json:"expires_at" doc:"Instante em que o ticket expira"`
	StreamURL string    `json:"stream_url" doc:"URL completa do stream, com o ticket já aplicado na query"`
}
