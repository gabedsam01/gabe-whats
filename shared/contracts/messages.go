// Package contracts define os DTOs compartilhados entre a API REST e o servidor MCP.
// Estas structs são a fonte única da verdade para a geração automática do OpenAPI
// (Huma/Swagger) e para a comunicação tipada entre apps/api e apps/mcp.
package contracts

import "time"

// MessageResponse é a resposta normalizada após enviar ou alterar uma mensagem no WhatsApp.
type MessageResponse struct {
	MessageID string    `json:"message_id" doc:"ID único da mensagem gerado ou retornado pelo WhatsApp"`
	ChatJID   string    `json:"chat_jid" doc:"JID normalizado do chat destinatário"`
	Timestamp time.Time `json:"timestamp" doc:"Timestamp oficial confirmado pelo servidor do WhatsApp"`
	Status    string    `json:"status" doc:"Estado da operação (ex: sent, edited, revoked, reacted)"`
}

// SendTextRequest define o payload para enviar mensagem de texto via REST ou MCP.
type SendTextRequest struct {
	To               string   `json:"to" required:"true" doc:"Número com DDI (ex: 5511999999999) ou JID completo (@s.whatsapp.net / @g.us)"`
	Text             string   `json:"text" required:"true" doc:"Conteúdo textual da mensagem"`
	ReplyToID        string   `json:"reply_to_id,omitempty" doc:"ID da mensagem original caso seja uma resposta citada"`
	Mentions         []string `json:"mentions,omitempty" doc:"Lista de JIDs ou números para mencionar no texto"`
	SimulateTypingMs int      `json:"simulate_typing_ms,omitempty" doc:"Tempo opcional (ms) simulando o status 'digitando...' antes do envio"`
}

// SendMediaRequest define o payload para enviar imagens, vídeos ou documentos.
type SendMediaRequest struct {
	To        string `json:"to" required:"true" doc:"Número ou JID do destinatário"`
	MediaType string `json:"media_type" required:"true" enum:"image,video,document" doc:"Tipo de mídia"`
	Source    string `json:"source" required:"true" doc:"URL HTTP, caminho local permitido ou string Base64"`
	MimeType  string `json:"mime_type,omitempty" doc:"MIME type explícito (ex: image/jpeg, application/pdf)"`
	Filename  string `json:"filename,omitempty" doc:"Nome do arquivo (recomendado para documentos)"`
	Caption   string `json:"caption,omitempty" doc:"Legenda opcional da mídia"`
	ReplyToID string `json:"reply_to_id,omitempty" doc:"ID da mensagem respondida"`
}

// SendVoiceRequest define o payload para enviar nota de voz push-to-talk (PTT).
type SendVoiceRequest struct {
	To        string `json:"to" required:"true" doc:"Número ou JID do destinatário"`
	Source    string `json:"source" required:"true" doc:"Áudio OGG/Opus via URL, caminho local ou Base64"`
	ReplyToID string `json:"reply_to_id,omitempty" doc:"ID da mensagem respondida"`
}

// ButtonType identifica os tipos de botão interativo NativeFlow suportados.
type ButtonType string

const (
	// ButtonQuickReply é um botão de resposta rápida que devolve o ID no webhook.
	ButtonQuickReply ButtonType = "quick_reply"
	// ButtonCTAURL abre uma URL externa no navegador do destinatário.
	ButtonCTAURL ButtonType = "cta_url"
	// ButtonCTACopy copia um código ou texto para a área de transferência.
	ButtonCTACopy ButtonType = "cta_copy"
)

// ButtonItem descreve um único botão interativo dentro de SendButtonsRequest.
type ButtonItem struct {
	Type  ButtonType `json:"type" required:"true" enum:"quick_reply,cta_url,cta_copy" doc:"Tipo do botão interativo"`
	Label string     `json:"label" required:"true" doc:"Texto exibido no botão"`
	ID    string     `json:"id" required:"true" doc:"Identificador retornado no webhook quando o botão é clicado"`
	Value string     `json:"value,omitempty" doc:"URL (para cta_url) ou código a copiar (para cta_copy)"`
}

// SendButtonsRequest define uma InteractiveMessage (NativeFlow) com botões.
type SendButtonsRequest struct {
	To      string       `json:"to" required:"true" doc:"Número ou JID do destinatário"`
	Header  string       `json:"header,omitempty" doc:"Cabeçalho opcional"`
	Body    string       `json:"body" required:"true" doc:"Corpo principal da mensagem"`
	Footer  string       `json:"footer,omitempty" doc:"Rodapé opcional"`
	Buttons []ButtonItem `json:"buttons" required:"true" minItems:"1" maxItems:"3" doc:"Lista de botões; o WhatsApp aceita no máximo 3"`
}

// ListRow representa uma opção selecionável dentro de uma ListSection.
type ListRow struct {
	RowID       string `json:"row_id" required:"true" doc:"ID único da linha retornado ao selecionar"`
	Title       string `json:"title" required:"true" doc:"Título da opção"`
	Description string `json:"description,omitempty" doc:"Descrição curta abaixo do título"`
}

// ListSection agrupa linhas dentro de um SendListRequest.
type ListSection struct {
	Title string    `json:"title" required:"true" doc:"Título da seção"`
	Rows  []ListRow `json:"rows" required:"true" minItems:"1" maxItems:"10" doc:"Opções desta seção; o WhatsApp aceita no máximo 10 linhas por seção"`
}

// SendListRequest define uma ListMessage com menu de opções expansíveis.
type SendListRequest struct {
	To          string        `json:"to" required:"true" doc:"Número ou JID do destinatário"`
	Title       string        `json:"title" required:"true" doc:"Título do menu"`
	Description string        `json:"description" required:"true" doc:"Texto explicativo da mensagem"`
	ButtonText  string        `json:"button_text" required:"true" doc:"Rótulo do botão que abre a lista"`
	Footer      string        `json:"footer,omitempty" doc:"Rodapé opcional"`
	Sections    []ListSection `json:"sections" required:"true" minItems:"1" maxItems:"10" doc:"Seções do menu; o WhatsApp aceita no máximo 10 seções"`
}

// SendPollRequest define um PollCreationMessage nativo do WhatsApp (100% compatível com conta pessoal).
type SendPollRequest struct {
	To            string   `json:"to" required:"true" doc:"Número ou JID do destinatário"`
	Question      string   `json:"question" required:"true" doc:"Pergunta da enquete"`
	Options       []string `json:"options" required:"true" minItems:"2" maxItems:"12" doc:"Opções clicáveis da enquete; o WhatsApp aceita de 2 a 12 opções"`
	MaxSelections *int     `json:"max_selections,omitempty" doc:"nil equivale a 1 (escolha única); 0 significa múltipla escolha; use 1..12 para limitar o número de seleções"`
}

// ReactionRequest adiciona ou remove uma reação de emoji em uma mensagem.
type ReactionRequest struct {
	ChatJID   string `json:"chat_jid" required:"true" doc:"JID do chat ou grupo"`
	SenderJID string `json:"sender_jid,omitempty" doc:"JID do remetente da mensagem alvo (obrigatório em grupos)"`
	MessageID string `json:"message_id" required:"true" doc:"ID da mensagem que receberá a reação"`
	Emoji     string `json:"emoji" required:"true" doc:"Emoji (ex: 👍) ou string vazia para remover a reação; o campo é obrigatório, mas aceita a string vazia"`
}

// EditMessageRequest substitui o texto de uma mensagem já enviada.
type EditMessageRequest struct {
	ChatJID string `json:"chat_jid" required:"true" doc:"JID do chat"`
	NewText string `json:"new_text" required:"true" doc:"Novo texto da mensagem"`
}

// RevokeMessageRequest apaga uma mensagem enviada para todos os participantes do chat.
// É o corpo de DELETE /api/v1/messages/{id}.
type RevokeMessageRequest struct {
	ChatJID string `json:"chat_jid" required:"true" doc:"JID do chat onde a mensagem será revogada"`
}

// MarkReadRequest marca uma ou mais mensagens recebidas como lidas (tique azul).
type MarkReadRequest struct {
	ChatJID    string     `json:"chat_jid" required:"true" doc:"JID do chat ou grupo onde estão as mensagens"`
	SenderJID  string     `json:"sender_jid,omitempty" doc:"JID do remetente das mensagens (obrigatório para recibo de leitura em grupos)"`
	MessageIDs []string   `json:"message_ids" required:"true" minItems:"1" maxItems:"50" doc:"IDs das mensagens que serão marcadas como lidas; até 50 por requisição"`
	Timestamp  *time.Time `json:"timestamp,omitempty" doc:"Instante de leitura; se omitido, a API usa o horário atual"`
}

// PresenceState identifica os estados de presença suportados no chat.
type PresenceState string

const (
	// PresenceTyping publica "digitando..." no chat.
	PresenceTyping PresenceState = "typing"
	// PresenceRecording publica "gravando áudio..." no chat.
	PresenceRecording PresenceState = "recording"
	// PresencePaused encerra o estado atual de digitação/gravação.
	PresencePaused PresenceState = "paused"
)

// PresenceRequest publica um estado de presença no chat.
type PresenceRequest struct {
	ChatJID string        `json:"chat_jid" required:"true" doc:"JID do chat ou grupo que receberá a presença"`
	State   PresenceState `json:"state" required:"true" enum:"typing,recording,paused" doc:"Estado a publicar: typing, recording ou paused"`
}

// MessageSummary é a visão enxuta de uma mensagem entregue no histórico recente.
type MessageSummary struct {
	MessageID   string    `json:"message_id" doc:"ID único da mensagem"`
	ChatJID     string    `json:"chat_jid" doc:"JID do chat ou grupo onde a mensagem está"`
	SenderJID   string    `json:"sender_jid" doc:"JID de quem enviou a mensagem"`
	SenderName  string    `json:"sender_name,omitempty" doc:"Nome de exibição do remetente, quando conhecido"`
	Timestamp   time.Time `json:"timestamp" doc:"Instante em que a mensagem foi enviada"`
	MessageType string    `json:"message_type" doc:"Tipo normalizado (ex: text, image, audio, video, document, sticker, poll, reaction, button_reply, list_reply)"`
	Text        string    `json:"text,omitempty" doc:"Texto extraível da mensagem (corpo, legenda, pergunta da enquete ou opção escolhida)"`
	IsGroup     bool      `json:"is_group" doc:"Se a mensagem pertence a um grupo"`
}

// RecentMessagesQuery define os parâmetros de query de GET /api/v1/messages/recent.
type RecentMessagesQuery struct {
	Limit   int    `json:"limit,omitempty" query:"limit" default:"50" maximum:"200" doc:"Quantidade máxima de mensagens a retornar; padrão 50, máximo 200"`
	ChatJID string `json:"chat_jid,omitempty" query:"chat_jid" doc:"Filtra por JID de chat ou grupo; se omitido, retorna as mensagens mais recentes de todos os chats"`
}

// RecentMessagesResponse lista as mensagens mais recentes, da mais nova para a mais antiga.
// É a resposta de GET /api/v1/messages/recent.
type RecentMessagesResponse struct {
	Messages []MessageSummary `json:"messages" doc:"Mensagens recentes encontradas"`
}
