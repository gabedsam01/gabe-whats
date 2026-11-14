package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gabedsam01/gabe-whats/shared/contracts"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// React adiciona uma reação de emoji em uma mensagem existente. Emoji vazio
// remove a reação anterior, conforme o protocolo do WhatsApp.
func (m *Manager) React(ctx context.Context, req contracts.ReactionRequest) (*contracts.MessageResponse, error) {
	chat, err := parseRecipient(req.ChatJID)
	if err != nil {
		return nil, invalidInput(err)
	}

	messageID := strings.TrimSpace(req.MessageID)
	if messageID == "" {
		return nil, invalidInput(errors.New("message_id vazio"))
	}

	sender, err := resolveReactionSender(chat, req.SenderJID)
	if err != nil {
		return nil, invalidInput(err)
	}

	message := m.client.BuildReaction(chat, sender, types.MessageID(messageID), strings.TrimSpace(req.Emoji))
	resp, err := m.client.SendMessage(ctx, chat, message)
	if err != nil {
		return nil, fmt.Errorf("enviar reação: %w", err)
	}
	return messageResponse(resp, chat, "reacted"), nil
}

// resolveReactionSender descobre o remetente da mensagem alvo. Em conversa
// individual o padrão é o próprio JID do chat; em grupos o campo é obrigatório.
func resolveReactionSender(chat types.JID, raw string) (types.JID, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		if chat.Server == types.GroupServer {
			return types.JID{}, errors.New("sender_jid é obrigatório para reagir a mensagens de grupo")
		}
		return chat, nil
	}

	sender, err := parseRecipient(value)
	if err != nil {
		return types.JID{}, fmt.Errorf("sender_jid inválido %q: %w", value, err)
	}
	return sender, nil
}

// EditMessage substitui o texto de uma mensagem já enviada. O WhatsApp aceita
// a edição apenas para mensagens próprias e dentro da janela de 20 minutos
// (whatsmeow.EditWindow); fora disso o erro do upstream é propagado.
func (m *Manager) EditMessage(ctx context.Context, messageID string, req contracts.EditMessageRequest) (*contracts.MessageResponse, error) {
	chat, err := parseRecipient(req.ChatJID)
	if err != nil {
		return nil, invalidInput(err)
	}

	id := strings.TrimSpace(messageID)
	if id == "" {
		return nil, invalidInput(errors.New("message_id vazio"))
	}
	if strings.TrimSpace(req.NewText) == "" {
		return nil, invalidInput(errors.New("new_text vazio"))
	}

	content := &waE2E.Message{Conversation: proto.String(req.NewText)}
	message := m.client.BuildEdit(chat, types.MessageID(id), content)
	resp, err := m.client.SendMessage(ctx, chat, message)
	if err != nil {
		return nil, fmt.Errorf("editar mensagem (apenas mensagens próprias e até %d minutos após o envio): %w", int(whatsmeow.EditWindow.Minutes()), err)
	}
	return messageResponse(resp, chat, "edited"), nil
}

// RevokeMessage apaga uma mensagem para todos os participantes do chat. Usa
// BuildRevoke + SendMessage porque o upstream marcou client.RevokeMessage como
// deprecado em favor desse caminho; o JID vazio indica a própria mensagem.
func (m *Manager) RevokeMessage(ctx context.Context, messageID string, req contracts.RevokeMessageRequest) (*contracts.MessageResponse, error) {
	chat, err := parseRecipient(req.ChatJID)
	if err != nil {
		return nil, invalidInput(err)
	}

	id := strings.TrimSpace(messageID)
	if id == "" {
		return nil, invalidInput(errors.New("message_id vazio"))
	}

	message := m.client.BuildRevoke(chat, types.EmptyJID, types.MessageID(id))
	resp, err := m.client.SendMessage(ctx, chat, message)
	if err != nil {
		return nil, fmt.Errorf("revogar mensagem: %w", err)
	}
	return messageResponse(resp, chat, "revoked"), nil
}

// MarkRead envia o recibo de leitura (tique azul) de até 50 mensagens do mesmo
// remetente. Sem timestamp explícito, usa o instante atual.
func (m *Manager) MarkRead(ctx context.Context, req contracts.MarkReadRequest) (*contracts.AckResponse, error) {
	chat, err := parseRecipient(req.ChatJID)
	if err != nil {
		return nil, invalidInput(err)
	}
	if len(req.MessageIDs) == 0 {
		return nil, invalidInput(errors.New("message_ids vazio"))
	}

	ids := make([]types.MessageID, 0, len(req.MessageIDs))
	for _, raw := range req.MessageIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, invalidInput(errors.New("message_ids contém um ID vazio"))
		}
		ids = append(ids, types.MessageID(id))
	}

	// Em conversa individual o remetente é o próprio chat; grupos podem
	// informar o participante autor das mensagens.
	sender := chat
	if value := strings.TrimSpace(req.SenderJID); value != "" {
		sender, err = parseRecipient(value)
		if err != nil {
			return nil, invalidInput(fmt.Errorf("sender_jid inválido %q: %w", value, err))
		}
	}

	timestamp := time.Now()
	if req.Timestamp != nil {
		timestamp = *req.Timestamp
	}

	if err := m.client.MarkRead(ctx, ids, timestamp, chat, sender); err != nil {
		return nil, fmt.Errorf("marcar mensagens como lidas: %w", err)
	}
	return &contracts.AckResponse{
		OK:     true,
		Detail: fmt.Sprintf("%d mensagem(ns) marcada(s) como lida(s) em %s", len(ids), chat.String()),
	}, nil
}

// SendPresence publica o estado de digitação/gravação no chat informado.
func (m *Manager) SendPresence(ctx context.Context, req contracts.PresenceRequest) (*contracts.AckResponse, error) {
	chat, err := parseRecipient(req.ChatJID)
	if err != nil {
		return nil, invalidInput(err)
	}

	var (
		state types.ChatPresence
		media types.ChatPresenceMedia
	)
	switch req.State {
	case contracts.PresenceTyping:
		state = types.ChatPresenceComposing
		media = types.ChatPresenceMediaText
	case contracts.PresenceRecording:
		state = types.ChatPresenceComposing
		media = types.ChatPresenceMediaAudio
	case contracts.PresencePaused:
		state = types.ChatPresencePaused
		media = types.ChatPresenceMediaText
	default:
		return nil, invalidInput(fmt.Errorf("state inválido %q: use typing, recording ou paused", req.State))
	}

	if err := m.client.SendChatPresence(ctx, chat, state, media); err != nil {
		return nil, fmt.Errorf("enviar presença: %w", err)
	}
	return &contracts.AckResponse{
		OK:     true,
		Detail: fmt.Sprintf("presença %q publicada em %s", req.State, chat.String()),
	}, nil
}
