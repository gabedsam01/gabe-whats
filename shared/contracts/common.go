package contracts

import "time"

// Problem é o corpo de erro padronizado pela RFC 9457 (Problem Details for HTTP APIs).
// Toda falha da API é devolvida neste formato com Content-Type application/problem+json,
// permitindo que agentes distingam validação (4xx) de falhas internas (5xx) pelo campo code.
type Problem struct {
	Type     string         `json:"type,omitempty" doc:"URI que identifica o tipo do problema (ex: about:blank ou https://api/problems/validation-error)"`
	Title    string         `json:"title,omitempty" doc:"Resumo curto e legível do tipo de problema"`
	Status   int            `json:"status,omitempty" doc:"Código de status HTTP gerado para esta ocorrência (ex: 400)"`
	Detail   string         `json:"detail,omitempty" doc:"Explicação legível e específica desta ocorrência do problema"`
	Instance string         `json:"instance,omitempty" doc:"URI que referencia a ocorrência específica, normalmente o caminho da requisição"`
	Code     string         `json:"code,omitempty" doc:"Código estável legível por máquina para o agente tratar o erro (ex: validation_error, not_found, whatsapp_limit_exceeded)"`
	Errors   []ProblemField `json:"errors,omitempty" doc:"Detalhes por campo quando o problema é uma falha de validação"`
}

// ProblemField detalha um erro de validação associado a um campo específico da requisição.
type ProblemField struct {
	Field   string `json:"field" doc:"Caminho do campo inválido no corpo da requisição (ex: buttons, options, name)"`
	Message string `json:"message" doc:"Descrição legível do motivo pelo qual o campo é inválido"`
}

// HealthResponse é a resposta do healthcheck do daemon.
type HealthResponse struct {
	Status        string `json:"status" doc:"Estado geral do daemon (ex: ok, degraded)"`
	Version       string `json:"version" doc:"Versão do binário do daemon da API"`
	UptimeSeconds int64  `json:"uptime_seconds" doc:"Tempo de atividade do processo em segundos"`
}

// AckResponse confirma uma operação que não devolve um recurso próprio.
// É a resposta de POST /api/v1/messages/read, POST /api/v1/messages/presence
// e POST /api/v1/groups/{jid}/leave.
type AckResponse struct {
	OK     bool   `json:"ok" doc:"Se true, a operação foi aplicada com sucesso"`
	Detail string `json:"detail,omitempty" doc:"Detalhe legível opcional sobre a operação aplicada"`
}

// MediaRef referencia uma mídia exposta pela API com URL temporária de download.
// O agente deve baixar o conteúdo antes de expires_at e guardá-lo no próprio contexto,
// pois a URL deixa de funcionar após o TTL (padrão de 15 minutos).
type MediaRef struct {
	ID          string    `json:"id" doc:"Identificador interno da mídia, usado em GET /api/v1/media/{id}"`
	MimeType    string    `json:"mime_type" doc:"MIME type do conteúdo (ex: image/jpeg, application/pdf)"`
	Filename    string    `json:"filename,omitempty" doc:"Nome de arquivo sugerido para o conteúdo"`
	SizeBytes   int64     `json:"size_bytes" doc:"Tamanho do conteúdo em bytes"`
	DownloadURL string    `json:"download_url" doc:"URL temporária de download servida por GET /api/v1/media/{id}"`
	ExpiresAt   time.Time `json:"expires_at" doc:"Instante em que a URL temporária deixa de ser válida"`
}
