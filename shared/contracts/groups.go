package contracts

import "time"

// CreateGroupRequest cria um novo grupo de WhatsApp via whatsmeow.CreateGroup.
type CreateGroupRequest struct {
	Name         string   `json:"name" required:"true" maxLength:"25" doc:"Nome do grupo (limite de 25 caracteres imposto pelo WhatsApp)"`
	Participants []string `json:"participants" required:"true" minItems:"1" maxItems:"1024" doc:"Lista de números ou JIDs dos participantes iniciais; o WhatsApp aceita até 1024 participantes por grupo"`
}

// GroupParticipantAction define as mutações permitidas em participantes de grupo.
type GroupParticipantAction string

const (
	// GroupActionAdd adiciona os participantes ao grupo.
	GroupActionAdd GroupParticipantAction = "add"
	// GroupActionRemove remove os participantes do grupo.
	GroupActionRemove GroupParticipantAction = "remove"
	// GroupActionPromote promove os participantes a administradores.
	GroupActionPromote GroupParticipantAction = "promote"
	// GroupActionDemote rebaixa os participantes de administradores.
	GroupActionDemote GroupParticipantAction = "demote"
)

// UpdateGroupParticipantsRequest adiciona, remove, promove ou rebaixa membros do grupo.
type UpdateGroupParticipantsRequest struct {
	Action       GroupParticipantAction `json:"action" required:"true" enum:"add,remove,promote,demote" doc:"Ação aplicada aos participantes"`
	Participants []string               `json:"participants" required:"true" minItems:"1" maxItems:"1024" doc:"Números ou JIDs alvo da ação; até 1024 por requisição"`
}

// GroupParticipantResult descreve o resultado da ação aplicada a um participante.
// O whatsmeow devolve o desfecho por participante, com erro individual (403 proibido,
// 409 conflito), por isso a resposta não é um único erro para a lista inteira.
type GroupParticipantResult struct {
	JID       string `json:"jid" doc:"JID do participante alvo da ação"`
	Status    string `json:"status" enum:"added,removed,promoted,demoted,failed" doc:"Resultado da ação para este participante: added, removed, promoted, demoted ou failed"`
	ErrorCode int    `json:"error_code,omitempty" doc:"Código numérico do erro individual do WhatsApp quando o status é failed (ex: 403 proibido, 409 conflito)"`
	Message   string `json:"message,omitempty" doc:"Descrição legível do erro individual quando o status é failed"`
}

// UpdateGroupParticipantsResponse devolve o resultado da ação participante a participante
// e, quando disponível, o estado atualizado do grupo.
type UpdateGroupParticipantsResponse struct {
	Action  GroupParticipantAction   `json:"action" enum:"add,remove,promote,demote" doc:"Ação aplicada aos participantes"`
	Results []GroupParticipantResult `json:"results" doc:"Resultado da ação para cada participante, na mesma ordem em que foram enviados"`
	Group   *GroupInfoResponse       `json:"group,omitempty" doc:"Estado atualizado do grupo; ausente quando a operação não devolve os metadados do grupo"`
}

// UpdateGroupSettingsRequest atualiza ou corrige metadados e permissões do grupo.
type UpdateGroupSettingsRequest struct {
	Name        *string `json:"name,omitempty" maxLength:"25" doc:"Novo nome do grupo (limite de 25 caracteres imposto pelo WhatsApp)"`
	Description *string `json:"description,omitempty" doc:"Nova descrição/tópico do grupo"`
	Announce    *bool   `json:"announce,omitempty" doc:"Se true, apenas administradores podem enviar mensagens"`
	Locked      *bool   `json:"locked,omitempty" doc:"Se true, apenas administradores podem editar os dados do grupo"`
}

// GroupParticipantInfo resume um participante retornado em GroupInfoResponse.
type GroupParticipantInfo struct {
	JID          string `json:"jid" doc:"JID do participante"`
	PhoneNumber  string `json:"phone_number,omitempty" doc:"Número de telefone do participante, quando disponível"`
	IsAdmin      bool   `json:"is_admin" doc:"Se o participante é administrador do grupo"`
	IsSuperAdmin bool   `json:"is_super_admin" doc:"Se o participante é o administrador criador do grupo"`
}

// GroupInfoResponse representa os metadados normalizados de um grupo de WhatsApp.
type GroupInfoResponse struct {
	JID          string                 `json:"jid" doc:"JID do grupo (termina em @g.us)"`
	Name         string                 `json:"name" doc:"Nome atual do grupo"`
	Description  string                 `json:"description,omitempty" doc:"Descrição/tópico do grupo"`
	OwnerJID     string                 `json:"owner_jid,omitempty" doc:"JID do criador/proprietário do grupo"`
	IsAnnounce   bool                   `json:"is_announce" doc:"Se true, apenas administradores podem enviar mensagens"`
	IsLocked     bool                   `json:"is_locked" doc:"Se true, apenas administradores podem editar os dados do grupo"`
	Participants []GroupParticipantInfo `json:"participants,omitempty" doc:"Participantes do grupo com seus papéis"`
}

// JoinGroupRequest entra em um grupo a partir de código ou URL de convite.
type JoinGroupRequest struct {
	Invite string `json:"invite" required:"true" doc:"Código do convite ou URL completa (ex: https://chat.whatsapp.com/XXXXXXXX)"`
}

// JoinGroupResponse confirma o grupo em que o número entrou.
type JoinGroupResponse struct {
	GroupJID string `json:"group_jid" doc:"JID do grupo em que o número entrou"`
}

// GroupsResponse lista os grupos dos quais o número participa.
type GroupsResponse struct {
	Groups []GroupInfoResponse `json:"groups" doc:"Grupos retornados pela API"`
}

// SetGroupPhotoRequest descreve os campos textuais do corpo multipart/form-data de
// PUT /api/v1/groups/{jid}/photo. O binário da imagem é enviado na parte "file" do
// formulário e não faz parte deste DTO.
type SetGroupPhotoRequest struct {
	Caption string `json:"caption,omitempty" form:"caption" doc:"Legenda opcional da foto, enviada no campo de formulário caption"`
}

// GroupPhotoResponse devolve a foto atualizada do grupo e a URL temporária de download.
type GroupPhotoResponse struct {
	JID         string    `json:"jid" doc:"JID do grupo atualizado"`
	PhotoID     string    `json:"photo_id" doc:"ID da foto retornado pelo WhatsApp"`
	DownloadURL string    `json:"download_url" doc:"URL temporária para baixar a foto via GET /api/v1/media/{id}"`
	ExpiresAt   time.Time `json:"expires_at" doc:"Instante em que a URL temporária de download expira"`
}

// InviteLinkResponse devolve o link de convite atual do grupo.
type InviteLinkResponse struct {
	JID   string `json:"jid" doc:"JID do grupo"`
	Link  string `json:"link" doc:"URL de convite atual do grupo"`
	Reset bool   `json:"reset" doc:"Se true, o link anterior foi revogado e este é um novo link"`
}
