// Package httpapi defines the REST route map and automatic OpenAPI/Swagger documentation layout.
package httpapi

// RouteBlueprint maps each HTTP endpoint to its responsibility and OpenAPI tag.
// Na fase de implementação, estas rotas serão registradas via Huma v2 (huma.Register)
// sobre Chi Router, gerando automaticamente:
//   - GET /docs          -> Interface interativa Scalar UI / Swagger UI
//   - GET /openapi.json  -> Spec OpenAPI 3.1 em JSON
//   - GET /openapi.yaml  -> Spec OpenAPI 3.1 em YAML
type RouteBlueprint struct {
	Method  string
	Path    string
	Tag     string
	Summary string
}

// PlannedRoutes lists the complete REST surface of apps/api.
var PlannedRoutes = []RouteBlueprint{
	// Health & Docs
	{Method: "GET", Path: "/health", Tag: "Health", Summary: "Healthcheck da API"},
	{Method: "GET", Path: "/docs", Tag: "Docs", Summary: "Documentação interativa automática (Swagger/Scalar UI)"},
	{Method: "GET", Path: "/openapi.json", Tag: "Docs", Summary: "Especificação OpenAPI 3.1 em JSON"},
	{Method: "GET", Path: "/openapi.yaml", Tag: "Docs", Summary: "Especificação OpenAPI 3.1 em YAML"},

	// Session
	{Method: "GET", Path: "/api/v1/session/status", Tag: "Session", Summary: "Status da conexão whatsmeow"},
	{Method: "GET", Path: "/api/v1/session/qr", Tag: "Session", Summary: "QR Code para pareamento"},
	{Method: "POST", Path: "/api/v1/session/pair-code", Tag: "Session", Summary: "Código de 8 dígitos (PairPhone)"},
	{Method: "POST", Path: "/api/v1/session/reconnect", Tag: "Session", Summary: "Reconectar WebSocket"},
	{Method: "POST", Path: "/api/v1/session/logout", Tag: "Session", Summary: "Deslogar dispositivo"},

	// Messages & Interactive
	{Method: "POST", Path: "/api/v1/messages/text", Tag: "Messages", Summary: "Enviar mensagem de texto"},
	{Method: "POST", Path: "/api/v1/messages/media", Tag: "Messages", Summary: "Enviar imagem, vídeo ou documento"},
	{Method: "POST", Path: "/api/v1/messages/voice", Tag: "Messages", Summary: "Enviar nota de voz (PTT)"},
	{Method: "POST", Path: "/api/v1/messages/buttons", Tag: "Messages", Summary: "Enviar botões interativos (NativeFlow)"},
	{Method: "POST", Path: "/api/v1/messages/list", Tag: "Messages", Summary: "Enviar menu de lista"},
	{Method: "POST", Path: "/api/v1/messages/poll", Tag: "Messages", Summary: "Enviar enquete interativa nativa"},
	{Method: "POST", Path: "/api/v1/messages/reaction", Tag: "Messages", Summary: "Reagir com emoji"},
	{Method: "PUT", Path: "/api/v1/messages/{id}", Tag: "Messages", Summary: "Editar mensagem enviada"},
	{Method: "DELETE", Path: "/api/v1/messages/{id}", Tag: "Messages", Summary: "Revogar mensagem para todos"},
	{Method: "POST", Path: "/api/v1/messages/read", Tag: "Messages", Summary: "Marcar mensagens como lidas"},
	{Method: "POST", Path: "/api/v1/messages/presence", Tag: "Messages", Summary: "Enviar status digitando/gravando"},
	{Method: "GET", Path: "/api/v1/messages/recent", Tag: "Messages", Summary: "Listar mensagens recentes (limit padrão 50, máximo 200)"},

	// Media
	{Method: "GET", Path: "/api/v1/media/{id}", Tag: "Media", Summary: "Baixar mídia temporária (TTL padrão de 15 minutos)"},

	// Groups
	{Method: "GET", Path: "/api/v1/groups", Tag: "Groups", Summary: "Listar grupos"},
	{Method: "POST", Path: "/api/v1/groups", Tag: "Groups", Summary: "Criar grupo"},
	{Method: "GET", Path: "/api/v1/groups/{jid}", Tag: "Groups", Summary: "Inspecionar grupo"},
	{Method: "PATCH", Path: "/api/v1/groups/{jid}", Tag: "Groups", Summary: "Editar/corrigir nome, descrição, anúncio ou trava"},
	{Method: "PUT", Path: "/api/v1/groups/{jid}/photo", Tag: "Groups", Summary: "Atualizar foto do grupo"},
	{Method: "POST", Path: "/api/v1/groups/{jid}/participants", Tag: "Groups", Summary: "Adicionar, remover, promover ou rebaixar membros"},
	{Method: "GET", Path: "/api/v1/groups/{jid}/invite-link", Tag: "Groups", Summary: "Obter ou resetar link de convite"},
	{Method: "POST", Path: "/api/v1/groups/join", Tag: "Groups", Summary: "Entrar em grupo por link"},
	{Method: "POST", Path: "/api/v1/groups/{jid}/leave", Tag: "Groups", Summary: "Sair do grupo"},

	// Contacts & Webhooks
	{Method: "POST", Path: "/api/v1/contacts/check", Tag: "Contacts", Summary: "Verificar números no WhatsApp"},
	{Method: "GET", Path: "/api/v1/webhooks", Tag: "Webhooks & Events", Summary: "Consultar configuração de webhook"},
	{Method: "PUT", Path: "/api/v1/webhooks", Tag: "Webhooks & Events", Summary: "Atualizar webhook e filtros"},
	{Method: "GET", Path: "/api/v1/events/stream", Tag: "Webhooks & Events", Summary: "Stream SSE em tempo real"},
}
