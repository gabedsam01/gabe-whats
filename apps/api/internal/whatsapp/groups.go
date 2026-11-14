package whatsapp

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/gabedsam01/gabe-whats/shared/contracts"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// ListGroups retorna os grupos aos quais a sessão conectada pertence.
func (m *Manager) ListGroups(ctx context.Context) (*contracts.GroupsResponse, error) {
	groups, err := m.client.GetJoinedGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar grupos: %w", err)
	}

	response := &contracts.GroupsResponse{Groups: make([]contracts.GroupInfoResponse, 0, len(groups))}
	for _, group := range groups {
		if converted := groupInfoToResponse(group); converted != nil {
			response.Groups = append(response.Groups, *converted)
		}
	}
	return response, nil
}

// GetGroup busca os metadados e os participantes de um grupo.
func (m *Manager) GetGroup(ctx context.Context, groupJID string) (*contracts.GroupInfoResponse, error) {
	jid, err := parseGroupJID(groupJID)
	if err != nil {
		return nil, invalidInput(err)
	}

	group, err := m.client.GetGroupInfo(ctx, jid)
	if err != nil {
		return nil, fmt.Errorf("buscar grupo: %w", err)
	}
	return groupInfoToResponse(group), nil
}

// CreateGroup cria um grupo e retorna seus metadados iniciais.
func (m *Manager) CreateGroup(ctx context.Context, req contracts.CreateGroupRequest) (*contracts.GroupInfoResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, invalidInput(fmt.Errorf("nome do grupo vazio"))
	}
	if utf8.RuneCountInString(name) > 25 {
		return nil, invalidInput(fmt.Errorf("o nome do grupo não pode exceder 25 caracteres"))
	}
	if len(req.Participants) == 0 {
		return nil, invalidInput(fmt.Errorf("informe ao menos um participante"))
	}

	participants := make([]types.JID, 0, len(req.Participants))
	for _, raw := range req.Participants {
		jid, err := parseRecipient(raw)
		if err != nil {
			return nil, invalidInput(err)
		}
		if jid.Server != types.DefaultUserServer && jid.Server != types.LegacyUserServer && jid.Server != types.HiddenUserServer {
			return nil, invalidInput(fmt.Errorf("o participante %q não é um JID de usuário", raw))
		}
		participants = append(participants, jid)
	}

	group, err := m.client.CreateGroup(ctx, whatsmeow.ReqCreateGroup{
		Name:         name,
		Participants: participants,
	})
	if err != nil {
		return nil, fmt.Errorf("criar grupo: %w", err)
	}
	return groupInfoToResponse(group), nil
}

// UpdateGroupParticipants atualiza participantes e preserva o resultado de cada um.
func (m *Manager) UpdateGroupParticipants(ctx context.Context, groupJID string, req contracts.UpdateGroupParticipantsRequest) (*contracts.UpdateGroupParticipantsResponse, error) {
	group, err := parseGroupJID(groupJID)
	if err != nil {
		return nil, invalidInput(err)
	}
	if len(req.Participants) == 0 {
		return nil, invalidInput(fmt.Errorf("informe ao menos um participante"))
	}

	var action whatsmeow.ParticipantChange
	switch req.Action {
	case contracts.GroupActionAdd:
		action = whatsmeow.ParticipantChangeAdd
	case contracts.GroupActionRemove:
		action = whatsmeow.ParticipantChangeRemove
	case contracts.GroupActionPromote:
		action = whatsmeow.ParticipantChangePromote
	case contracts.GroupActionDemote:
		action = whatsmeow.ParticipantChangeDemote
	default:
		return nil, invalidInput(fmt.Errorf("ação de participante inválida: %q", req.Action))
	}

	participants := make([]types.JID, 0, len(req.Participants))
	for _, raw := range req.Participants {
		jid, err := parseRecipient(raw)
		if err != nil {
			return nil, invalidInput(err)
		}
		if jid.Server != types.DefaultUserServer && jid.Server != types.LegacyUserServer && jid.Server != types.HiddenUserServer {
			return nil, invalidInput(fmt.Errorf("o participante %q não é um JID de usuário", raw))
		}
		participants = append(participants, jid)
	}

	updated, err := m.client.UpdateGroupParticipants(ctx, group, participants, action)
	if err != nil {
		return nil, fmt.Errorf("atualizar participantes do grupo: %w", err)
	}

	resultStatus := map[contracts.GroupParticipantAction]string{
		contracts.GroupActionAdd:     "added",
		contracts.GroupActionRemove:  "removed",
		contracts.GroupActionPromote: "promoted",
		contracts.GroupActionDemote:  "demoted",
	}
	response := &contracts.UpdateGroupParticipantsResponse{
		Action:  req.Action,
		Results: make([]contracts.GroupParticipantResult, 0, len(updated)),
	}
	for i, participant := range updated {
		jid := participant.JID.String()
		if jid == "" && i < len(participants) {
			jid = participants[i].String()
		}
		result := contracts.GroupParticipantResult{JID: jid, Status: resultStatus[req.Action]}
		if participant.Error != 0 {
			result.Status = "failed"
			result.ErrorCode = int(participant.Error)
			result.Message = participantErrorMessage(participant.Error)
		}
		response.Results = append(response.Results, result)
	}
	if refreshed, err := m.client.GetGroupInfo(ctx, group); err == nil {
		response.Group = groupInfoToResponse(refreshed)
	}
	return response, nil
}

// UpdateGroupSettings aplica as alterações de metadados fornecidas.
func (m *Manager) UpdateGroupSettings(ctx context.Context, groupJID string, req contracts.UpdateGroupSettingsRequest) error {
	group, err := parseGroupJID(groupJID)
	if err != nil {
		return invalidInput(err)
	}
	if req.Name == nil && req.Description == nil && req.Announce == nil && req.Locked == nil {
		return invalidInput(fmt.Errorf("informe ao menos uma configuração para atualizar"))
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return invalidInput(fmt.Errorf("nome do grupo vazio"))
		}
		if utf8.RuneCountInString(name) > 25 {
			return invalidInput(fmt.Errorf("o nome do grupo não pode exceder 25 caracteres"))
		}
		if err := m.client.SetGroupName(ctx, group, name); err != nil {
			return fmt.Errorf("atualizar nome do grupo: %w", err)
		}
	}
	if req.Description != nil {
		if err := m.client.SetGroupTopic(ctx, group, "", "", *req.Description); err != nil {
			return fmt.Errorf("atualizar descrição do grupo: %w", err)
		}
	}
	if req.Announce != nil {
		if err := m.client.SetGroupAnnounce(ctx, group, *req.Announce); err != nil {
			return fmt.Errorf("atualizar modo de anúncio do grupo: %w", err)
		}
	}
	if req.Locked != nil {
		if err := m.client.SetGroupLocked(ctx, group, *req.Locked); err != nil {
			return fmt.Errorf("atualizar bloqueio do grupo: %w", err)
		}
	}
	return nil
}

// SetGroupPhoto atualiza a foto e a disponibiliza pelo armazenamento de mídia.
func (m *Manager) SetGroupPhoto(ctx context.Context, groupJID string, data []byte, mimeType string, caption string) (*contracts.GroupPhotoResponse, error) {
	group, err := parseGroupJID(groupJID)
	if err != nil {
		return nil, invalidInput(err)
	}
	if len(data) == 0 {
		return nil, invalidInput(fmt.Errorf("arquivo de foto vazio"))
	}
	if len(data) > maxSourceBytes {
		return nil, invalidInput(fmt.Errorf("arquivo de foto excede o limite de %d bytes", maxSourceBytes))
	}
	if m.media == nil {
		return nil, fmt.Errorf("armazenamento de mídia não configurado")
	}

	photoID, err := m.client.SetGroupPhoto(ctx, group, data)
	if err != nil {
		return nil, fmt.Errorf("atualizar foto do grupo: %w", err)
	}
	ref, err := m.media.Save(ctx, data, mimeType, "group-photo.jpg")
	if err != nil {
		return nil, fmt.Errorf("armazenar foto do grupo: %w", err)
	}
	return &contracts.GroupPhotoResponse{
		JID:         group.String(),
		PhotoID:     photoID,
		DownloadURL: ref.DownloadURL,
		ExpiresAt:   ref.ExpiresAt,
	}, nil
}

// GetGroupInviteLink retorna o link atual, revogando-o antes se solicitado.
func (m *Manager) GetGroupInviteLink(ctx context.Context, groupJID string, reset bool) (*contracts.InviteLinkResponse, error) {
	group, err := parseGroupJID(groupJID)
	if err != nil {
		return nil, invalidInput(err)
	}
	link, err := m.client.GetGroupInviteLink(ctx, group, reset)
	if err != nil {
		return nil, fmt.Errorf("buscar link de convite do grupo: %w", err)
	}
	return &contracts.InviteLinkResponse{JID: group.String(), Link: link, Reset: reset}, nil
}

// JoinGroupWithLink entra em um grupo usando um código ou link de convite.
func (m *Manager) JoinGroupWithLink(ctx context.Context, invite string) (*contracts.JoinGroupResponse, error) {
	code, err := parseGroupInvite(invite)
	if err != nil {
		return nil, invalidInput(err)
	}
	group, err := m.client.JoinGroupWithLink(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("entrar no grupo pelo convite: %w", err)
	}
	return &contracts.JoinGroupResponse{GroupJID: group.String()}, nil
}

// LeaveGroup remove a sessão do grupo informado.
func (m *Manager) LeaveGroup(ctx context.Context, groupJID string) error {
	group, err := parseGroupJID(groupJID)
	if err != nil {
		return invalidInput(err)
	}
	if err := m.client.LeaveGroup(ctx, group); err != nil {
		return fmt.Errorf("sair do grupo: %w", err)
	}
	return nil
}

func parseGroupJID(raw string) (types.JID, error) {
	jid, err := parseRecipient(raw)
	if err != nil {
		return types.JID{}, err
	}
	if jid.Server != types.GroupServer {
		return types.JID{}, fmt.Errorf("JID de grupo inválido: %q", raw)
	}
	return jid, nil
}

func parseGroupInvite(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("convite vazio")
	}
	if !strings.Contains(value, "://") {
		return strings.Trim(strings.TrimSpace(value), "/"), nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("URL de convite inválida: %w", err)
	}
	if parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "chat.whatsapp.com") || parsed.User != nil {
		return "", fmt.Errorf("URL de convite inválida")
	}
	code := strings.Trim(parsed.Path, "/")
	if code == "" || strings.Contains(code, "/") {
		return "", fmt.Errorf("código de convite inválido")
	}
	return code, nil
}

func participantErrorMessage(code int) string {
	switch code {
	case 403:
		return "operação não permitida"
	case 404:
		return "participante não encontrado"
	case 409:
		return "conflito na operação do participante"
	default:
		return fmt.Sprintf("falha ao atualizar participante (código %d)", code)
	}
}

func groupInfoToResponse(group *types.GroupInfo) *contracts.GroupInfoResponse {
	if group == nil {
		return nil
	}
	response := &contracts.GroupInfoResponse{
		JID:          group.JID.String(),
		Name:         group.GroupName.Name,
		Description:  group.GroupTopic.Topic,
		OwnerJID:     group.OwnerJID.String(),
		IsAnnounce:   group.GroupAnnounce.IsAnnounce,
		IsLocked:     group.GroupLocked.IsLocked,
		Participants: make([]contracts.GroupParticipantInfo, 0, len(group.Participants)),
	}
	for _, participant := range group.Participants {
		jid := participant.JID
		if jid.IsEmpty() {
			jid = participant.PhoneNumber
		}
		response.Participants = append(response.Participants, contracts.GroupParticipantInfo{
			JID:          jid.String(),
			PhoneNumber:  participant.PhoneNumber.String(),
			IsAdmin:      participant.IsAdmin,
			IsSuperAdmin: participant.IsSuperAdmin,
		})
	}
	return response
}
