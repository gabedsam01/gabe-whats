// Package tools defines the blueprint of all MCP Tools exposed to AI agents.
package tools

// MCPToolBlueprint documents one MCP tool, its target REST call, and its purpose.
type MCPToolBlueprint struct {
	Name        string
	RESTMethod  string
	RESTPath    string
	Description string
}

// PlannedTools enumerates every tool that apps/mcp will register for agents.
var PlannedTools = []MCPToolBlueprint{
	// Sessão & Contatos
	{Name: "whatsapp_session_status", RESTMethod: "GET", RESTPath: "/api/v1/session/status", Description: "Consulta se o número pessoal está conectado e logado no WhatsApp"},
	{Name: "whatsapp_get_qr", RESTMethod: "GET", RESTPath: "/api/v1/session/qr", Description: "Obtém QR Code de pareamento quando não conectado"},
	{Name: "whatsapp_pair_phone", RESTMethod: "POST", RESTPath: "/api/v1/session/pair-code", Description: "Gera código de pareamento de 8 dígitos por telefone"},
	{Name: "whatsapp_check_numbers", RESTMethod: "POST", RESTPath: "/api/v1/contacts/check", Description: "Verifica se números de telefone possuem conta no WhatsApp"},

	// Mensagens & Interatividade
	{Name: "whatsapp_send_text", RESTMethod: "POST", RESTPath: "/api/v1/messages/text", Description: "Envia mensagem de texto (opcionalmente respondendo/mencionando)"},
	{Name: "whatsapp_send_media", RESTMethod: "POST", RESTPath: "/api/v1/messages/media", Description: "Envia imagem, vídeo ou documento com legenda opcional"},
	{Name: "whatsapp_send_voice_ptt", RESTMethod: "POST", RESTPath: "/api/v1/messages/voice", Description: "Envia áudio como nota de voz gravada (PTT)"},
	{Name: "whatsapp_send_buttons", RESTMethod: "POST", RESTPath: "/api/v1/messages/buttons", Description: "Envia botões interativos (quick_reply, cta_url, cta_copy)"},
	{Name: "whatsapp_send_list", RESTMethod: "POST", RESTPath: "/api/v1/messages/list", Description: "Envia menu de lista selecionável"},
	{Name: "whatsapp_send_poll", RESTMethod: "POST", RESTPath: "/api/v1/messages/poll", Description: "Envia enquete interativa nativa (100% compatível com uso pessoal)"},
	{Name: "whatsapp_react_message", RESTMethod: "POST", RESTPath: "/api/v1/messages/reaction", Description: "Reage com emoji a uma mensagem"},
	{Name: "whatsapp_edit_message", RESTMethod: "PUT", RESTPath: "/api/v1/messages/{id}", Description: "Edita uma mensagem já enviada"},
	{Name: "whatsapp_revoke_message", RESTMethod: "DELETE", RESTPath: "/api/v1/messages/{id}", Description: "Apaga/revoga mensagem para todos"},
	{Name: "whatsapp_mark_read", RESTMethod: "POST", RESTPath: "/api/v1/messages/read", Description: "Marca mensagens recebidas como lidas"},

	// Gestão de Grupos
	{Name: "whatsapp_list_groups", RESTMethod: "GET", RESTPath: "/api/v1/groups", Description: "Lista todos os grupos que o número participa"},
	{Name: "whatsapp_get_group", RESTMethod: "GET", RESTPath: "/api/v1/groups/{jid}", Description: "Obtém detalhes, regras e participantes de um grupo"},
	{Name: "whatsapp_create_group", RESTMethod: "POST", RESTPath: "/api/v1/groups", Description: "Cria um novo grupo com participantes"},
	{Name: "whatsapp_update_group_participants", RESTMethod: "POST", RESTPath: "/api/v1/groups/{jid}/participants", Description: "Adiciona, remove, promove ou rebaixa participantes"},
	{Name: "whatsapp_update_group_settings", RESTMethod: "PATCH", RESTPath: "/api/v1/groups/{jid}", Description: "Edita/corrige nome, descrição, modo anúncio ou trava do grupo"},
	{Name: "whatsapp_get_group_invite_link", RESTMethod: "GET", RESTPath: "/api/v1/groups/{jid}/invite-link", Description: "Obtém ou reseta o link de convite do grupo"},
	{Name: "whatsapp_join_group", RESTMethod: "POST", RESTPath: "/api/v1/groups/join", Description: "Entra em um grupo via link/código de convite"},
	{Name: "whatsapp_leave_group", RESTMethod: "POST", RESTPath: "/api/v1/groups/{jid}/leave", Description: "Sai de um grupo"},
}
