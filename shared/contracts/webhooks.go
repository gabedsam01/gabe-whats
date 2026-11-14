package contracts

import "time"

// WebhookEventType identifica um tipo de evento enviado por webhook.
type WebhookEventType string

const (
	EventMessageReceived     WebhookEventType = "message.received"
	EventGroupUpdated        WebhookEventType = "group.updated"
	EventSessionConnected    WebhookEventType = "session.connected"
	EventSessionDisconnected WebhookEventType = "session.disconnected"
	EventSessionLoggedOut    WebhookEventType = "session.logged_out"
)

// WebhookMessageType identifica o tipo normalizado de uma mensagem recebida.
type WebhookMessageType string

const (
	WebhookMessageTypeText        WebhookMessageType = "text"
	WebhookMessageTypeImage       WebhookMessageType = "image"
	WebhookMessageTypeAudio       WebhookMessageType = "audio"
	WebhookMessageTypeVideo       WebhookMessageType = "video"
	WebhookMessageTypeDocument    WebhookMessageType = "document"
	WebhookMessageTypeSticker     WebhookMessageType = "sticker"
	WebhookMessageTypeLocation    WebhookMessageType = "location"
	WebhookMessageTypeContact     WebhookMessageType = "contact"
	WebhookMessageTypeButtonReply WebhookMessageType = "button_reply"
	WebhookMessageTypeListReply   WebhookMessageType = "list_reply"
	WebhookMessageTypePoll        WebhookMessageType = "poll"
	WebhookMessageTypePollVote    WebhookMessageType = "poll_vote"
	WebhookMessageTypeReaction    WebhookMessageType = "reaction"
	WebhookMessageTypeRevoke      WebhookMessageType = "revoke"
	WebhookMessageTypeUnknown     WebhookMessageType = "unknown"
)

// WebhookGroupChangeType identifica uma alteração nos dados ou participantes do grupo.
type WebhookGroupChangeType string

const (
	GroupChangeSubject                WebhookGroupChangeType = "subject"
	GroupChangeDescription            WebhookGroupChangeType = "description"
	GroupChangeAnnounce               WebhookGroupChangeType = "announce"
	GroupChangeLocked                 WebhookGroupChangeType = "locked"
	GroupChangeParticipantAdd         WebhookGroupChangeType = "participant_add"
	GroupChangeParticipantRemove      WebhookGroupChangeType = "participant_remove"
	GroupChangeParticipantPromote     WebhookGroupChangeType = "participant_promote"
	GroupChangeParticipantDemote      WebhookGroupChangeType = "participant_demote"
	GroupChangeEphemeral              WebhookGroupChangeType = "ephemeral"
	GroupChangeDelete                 WebhookGroupChangeType = "delete"
	GroupChangeMembershipApprovalMode WebhookGroupChangeType = "membership_approval_mode"
	GroupChangeOther                  WebhookGroupChangeType = "other"
)

// WebhookButtonReply normaliza cliques em botões ou linhas de listas.
type WebhookButtonReply struct {
	SelectedID   string `json:"selected_id" doc:"Identificador da opção selecionada"`
	SelectedText string `json:"selected_text,omitempty" doc:"Texto exibido na opção selecionada"`
	ResponseJSON string `json:"response_json,omitempty" doc:"JSON de parâmetros de um clique NativeFlow, recebido pelo upstream em InteractiveResponseMessage.ParamsJSON"`
}

// WebhookPollVote normaliza um voto de enquete usando os textos recuperados do cache SHA-256.
// A remoção de voto também chega com SelectedOptions vazio; texto ausente não indica remoção.
type WebhookPollVote struct {
	PollMessageID   string   `json:"poll_message_id" doc:"ID da mensagem que contém a enquete"`
	Question        string   `json:"question,omitempty" doc:"Vazio quando o cache não possui a criação da enquete"`
	SelectedOptions []string `json:"selected_options,omitempty" doc:"Vazio quando o cache não possui a criação da enquete; votos de remoção também chegam vazios"`
	SelectedHashes  []string `json:"selected_hashes" doc:"Hashes SHA-256 hexadecimais das opções selecionadas; sempre presente, inclusive em remoções de voto"`
	VoterJID        string   `json:"voter_jid" doc:"JID de quem votou na enquete"`
	PollSenderJID   string   `json:"poll_sender_jid,omitempty" doc:"JID de quem criou a enquete, quando disponível"`
}

// WebhookMediaPayload contém metadados e referências ao conteúdo de uma mídia recebida.
type WebhookMediaPayload struct {
	MediaID     string     `json:"media_id,omitempty" doc:"ID para solicitar novamente a mídia em GET /api/v1/media/{id}, inclusive após a URL temporária retornar 410 Gone"`
	MimeType    string     `json:"mime_type" doc:"Tipo MIME do arquivo de mídia"`
	Filename    string     `json:"filename,omitempty" doc:"Nome do arquivo, quando disponível"`
	IsPTT       bool       `json:"is_ptt,omitempty" doc:"Indica se o áudio foi enviado como mensagem de voz"`
	FileSize    int64      `json:"file_size" doc:"Tamanho do arquivo em bytes"`
	Width       int        `json:"width,omitempty" doc:"Largura da mídia em pixels, quando disponível"`
	Height      int        `json:"height,omitempty" doc:"Altura da mídia em pixels, quando disponível"`
	Seconds     int        `json:"seconds,omitempty" doc:"Duração da mídia em segundos, quando disponível"`
	Caption     string     `json:"caption,omitempty" doc:"Legenda da mídia, quando disponível"`
	IsAnimated  bool       `json:"is_animated,omitempty" doc:"Indica se a mídia é animada"`
	DownloadURL string     `json:"download_url,omitempty" doc:"URL temporária para baixar a mídia; presente somente quando o gateway armazenou o arquivo (AutoDownloadMedia), com TTL padrão de 15 minutos"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty" doc:"Expiração da URL; presente somente quando o gateway armazenou o arquivo (AutoDownloadMedia)"`
	LocalPath   string     `json:"local_path,omitempty" doc:"Caminho local opcional do arquivo de mídia"`
	Base64      string     `json:"base64,omitempty" doc:"Conteúdo opcional do arquivo codificado em Base64"`
}

// WebhookReaction descreve uma reação a uma mensagem.
// O upstream entrega ReactionMessage com Key da mensagem-alvo; o emoji é obtido via DecryptReaction.
type WebhookReaction struct {
	Emoji           string `json:"emoji" doc:"Emoji da reação; string vazia indica remoção da reação"`
	TargetMessageID string `json:"target_message_id" doc:"ID da mensagem que recebeu a reação"`
	TargetSenderJID string `json:"target_sender_jid" doc:"JID do remetente da mensagem que recebeu a reação"`
	ByMe            bool   `json:"by_me" doc:"Indica se a reação foi enviada pelo próprio número"`
}

// WebhookRevoke descreve a mensagem revogada.
// O whatsmeow entrega a revogação como ProtocolMessage REVOKE, sem unwrap; o normalizador traduz para message_type revoke.
type WebhookRevoke struct {
	TargetMessageID string `json:"target_message_id" doc:"ID da mensagem revogada"`
	TargetSenderJID string `json:"target_sender_jid" doc:"JID do remetente da mensagem revogada"`
}

// WebhookPollCreation descreve a criação de uma enquete recebida.
// Este evento ancora o cache SHA-256 para texto; toda enquete recebida deve ser cacheada.
type WebhookPollCreation struct {
	PollMessageID         string   `json:"poll_message_id" doc:"ID da mensagem que contém a enquete"`
	Question              string   `json:"question" doc:"Pergunta legível da enquete"`
	Options               []string `json:"options" doc:"Textos das opções legíveis da enquete"`
	SelectableOptionCount int      `json:"selectable_option_count" doc:"Quantidade de opções que podem ser selecionadas por voto"`
	PollSenderJID         string   `json:"poll_sender_jid" doc:"JID de quem criou a enquete"`
}

// WebhookInboundEvent é o evento normalizado enviado a agentes por Webhook HTTP POST ou SSE.
// O webhook envia sempre este envelope; event_type discrimina o conteúdo e os objetos
// group/session só aparecem nos eventos correspondentes.
type WebhookInboundEvent struct {
	EventID      string               `json:"event_id" doc:"Identificador único deste evento"`
	EventType    WebhookEventType     `json:"event_type" enum:"message.received,group.updated,session.connected,session.disconnected,session.logged_out" doc:"Tipo do evento enviado"`
	Timestamp    time.Time            `json:"timestamp" doc:"Data e hora em que o evento ocorreu"`
	ChatJID      string               `json:"chat_jid" doc:"JID do chat relacionado ao evento"`
	SenderJID    string               `json:"sender_jid" doc:"JID de quem enviou a mensagem"`
	SenderName   string               `json:"sender_name,omitempty" doc:"Nome de exibição de quem enviou a mensagem"`
	ServerID     string               `json:"server_id,omitempty" doc:"Identificador da mensagem atribuído pelo servidor do WhatsApp"`
	VerifiedName string               `json:"verified_name,omitempty" doc:"Nome comercial verificado associado ao remetente"`
	IsGroup      bool                 `json:"is_group" doc:"Indica se a mensagem foi recebida em um grupo"`
	IsFromMe     bool                 `json:"is_from_me" doc:"Indica se a mensagem foi enviada pelo próprio número"`
	MessageID    string               `json:"message_id,omitempty" doc:"Identificador da mensagem"`
	MessageType  WebhookMessageType   `json:"message_type,omitempty" enum:"text,image,audio,video,document,sticker,location,contact,button_reply,list_reply,poll,poll_vote,reaction,revoke,unknown" doc:"Tipo normalizado da mensagem"`
	Text         string               `json:"text,omitempty" doc:"Texto da mensagem, quando disponível"`
	IsEdit       bool                 `json:"is_edit" doc:"Indica se o evento corresponde a uma edição de mensagem"`
	EditedText   string               `json:"edited_text,omitempty" doc:"Texto atualizado da mensagem editada"`
	ReplyToID    string               `json:"reply_to_id,omitempty" doc:"Identificador da mensagem respondida"`
	ButtonReply  *WebhookButtonReply  `json:"button_reply,omitempty" doc:"Detalhes da opção de botão ou lista selecionada"`
	PollVote     *WebhookPollVote     `json:"poll_vote,omitempty" doc:"Detalhes do voto em uma enquete"`
	Media        *WebhookMediaPayload `json:"media,omitempty" doc:"Metadados e conteúdo ou URL temporária da mídia"`
	Reaction     *WebhookReaction     `json:"reaction,omitempty" doc:"Detalhes da reação recebida"`
	Revoke       *WebhookRevoke       `json:"revoke,omitempty" doc:"Detalhes da mensagem revogada"`
	Poll         *WebhookPollCreation `json:"poll,omitempty" doc:"Detalhes da enquete recebida; âncora do cache de hashes SHA-256 para textos de opção"`
	Group        *WebhookGroupEvent   `json:"group,omitempty" doc:"Alteração delta do grupo; presente apenas quando event_type é group.updated"`
	Session      *WebhookSessionEvent `json:"session,omitempty" doc:"Estado da sessão; presente apenas nos eventos session.connected, session.disconnected e session.logged_out"`
}

// WebhookGroupChange descreve uma alteração de metadados ou participantes de um grupo.
type WebhookGroupChange struct {
	Type         WebhookGroupChangeType `json:"type" enum:"subject,description,announce,locked,participant_add,participant_remove,participant_promote,participant_demote,ephemeral,delete,membership_approval_mode,other" doc:"Tipo da alteração no grupo; other mapeia UnknownChanges do upstream"`
	JID          string                 `json:"jid,omitempty" doc:"JID do participante relacionado à alteração, quando aplicável"`
	Participants []string               `json:"participants,omitempty" doc:"JIDs dos participantes relacionados à alteração, quando aplicável"`
}

// WebhookGroupEvent descreve somente o delta de uma atualização de grupo, não um snapshot.
// O tipo do evento vem do envelope (WebhookInboundEvent.event_type = group.updated).
type WebhookGroupEvent struct {
	JID              string               `json:"jid" doc:"JID do grupo atualizado"`
	SenderJID        string               `json:"sender_jid" doc:"JID de quem causou a alteração no grupo"`
	Name             *string              `json:"name,omitempty" doc:"Novo nome do grupo, somente quando alterado"`
	Topic            *string              `json:"topic,omitempty" doc:"Novo tópico do grupo, somente quando alterado"`
	IsAnnounce       *bool                `json:"is_announce,omitempty" doc:"Novo estado de envio restrito a administradores, somente quando alterado"`
	IsLocked         *bool                `json:"is_locked,omitempty" doc:"Novo estado de edição restrita a administradores, somente quando alterado"`
	ParticipantCount *int                 `json:"participant_count,omitempty" doc:"Preenchido apenas quando disponível; o evento de grupo não carrega esse número"`
	Changes          []WebhookGroupChange `json:"changes" doc:"Alterações delta que ocorreram no grupo"`
}

// WebhookSessionEvent descreve uma mudança no estado da sessão do WhatsApp.
// No upstream, events.Connected e events.Disconnected são structs vazias; os dados vêm do store.
// Reason só existe em LoggedOut.
type WebhookSessionEvent struct {
	JID      string `json:"jid,omitempty" doc:"JID da conta conectada à sessão, obtido do store quando disponível"`
	PushName string `json:"push_name,omitempty" doc:"Nome de exibição da conta"`
	Platform string `json:"platform,omitempty" doc:"Plataforma ou dispositivo associado à sessão"`
	Reason   string `json:"reason,omitempty" doc:"Motivo disponível somente no evento LoggedOut"`
}

// WebhookConfig define o destino do webhook e as regras de filtragem para uso por agentes pessoais.
type WebhookConfig struct {
	URL               string   `json:"url" doc:"URI HTTP(S) absoluta de destino do agente (OpenClaw / Hermes / etc.)"`
	Secret            string   `json:"secret,omitempty" doc:"write-only: nunca e retornado; assina o corpo com HMAC-SHA256 no header X-GabeWhats-Signature com valor sha256=<hex>"`
	IgnoreFromMe      bool     `json:"ignore_from_me" doc:"Se true, filtra eventos message.received de mensagens enviadas pelo próprio número para evitar loops"`
	AllowJIDs         []string `json:"allow_jids,omitempty" doc:"Allowlist opcional de chats/números que filtra quais eventos message.received acionam o agente"`
	AutoDownloadMedia bool     `json:"auto_download_media" doc:"Se true, baixa e descriptografa mídias/áudios automaticamente"`
	IncludeBase64     bool     `json:"include_base64" doc:"Se true, inclui o conteúdo em Base64 no payload do webhook"`
}

// WebhookConfigResponse é a configuração devolvida pelo GET /api/v1/webhooks e pelo tool MCP, sem expor o segredo.
type WebhookConfigResponse struct {
	URL               string   `json:"url" doc:"URI HTTP(S) absoluta de destino do agente (OpenClaw / Hermes / etc.)"`
	IgnoreFromMe      bool     `json:"ignore_from_me" doc:"Se true, filtra eventos message.received de mensagens enviadas pelo próprio número para evitar loops"`
	AllowJIDs         []string `json:"allow_jids,omitempty" doc:"Allowlist opcional de chats/números que filtra quais eventos message.received acionam o agente"`
	AutoDownloadMedia bool     `json:"auto_download_media" doc:"Se true, baixa e descriptografa mídias/áudios automaticamente"`
	IncludeBase64     bool     `json:"include_base64" doc:"Se true, inclui o conteúdo em Base64 no payload do webhook"`
	SecretSet         bool     `json:"secret_set" doc:"Indica se já existe segredo configurado, sem revelá-lo"`
}
