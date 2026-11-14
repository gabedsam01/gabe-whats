package whatsapp

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gabedsam01/gabe-whats/shared/contracts"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// handleEvent normaliza eventos do whatsmeow para o contrato compartilhado e os
// publica no barramento (histórico, SSE e webhook).
func (m *Manager) handleEvent(raw any) {
	switch evt := raw.(type) {
	case *events.Connected:
		m.client.Log.Infof("conectado ao WhatsApp")
		m.publishSession(contracts.EventSessionConnected, "")
	case *events.Disconnected:
		m.client.Log.Warnf("desconectado do WhatsApp")
		m.publishSession(contracts.EventSessionDisconnected, "")
	case *events.LoggedOut:
		m.client.Log.Warnf("sessão encerrada pelo WhatsApp")
		m.publishSession(contracts.EventSessionLoggedOut, evt.Reason.String())
	case *events.GroupInfo:
		m.publishGroupEvent(evt)
	case *events.Message:
		if m.publisher == nil {
			return
		}
		// Download de mídia pode demorar; nunca bloqueie a fila de eventos.
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if event := m.normalizeMessage(ctx, evt); event != nil {
				m.publisher.Publish(ctx, *event)
			}
		}()
	}
}

// publishSession publica um evento de sessão.
func (m *Manager) publishSession(eventType contracts.WebhookEventType, reason string) {
	if m.publisher == nil {
		return
	}
	event := contracts.WebhookInboundEvent{
		EventID:   fmt.Sprintf("sess-%d", time.Now().UnixNano()),
		EventType: eventType,
		Timestamp: time.Now(),
		Session: &contracts.WebhookSessionEvent{
			Platform: m.client.Store.Platform,
			PushName: m.client.Store.PushName,
			Reason:   reason,
		},
	}
	if m.client.Store.ID != nil {
		event.Session.JID = m.client.Store.ID.String()
	}
	m.publisher.Publish(context.Background(), event)
}

// normalizeMessage converte *events.Message em WebhookInboundEvent.
func (m *Manager) normalizeMessage(ctx context.Context, evt *events.Message) *contracts.WebhookInboundEvent {
	if evt == nil || evt.Info.ID == "" {
		return nil
	}

	event := &contracts.WebhookInboundEvent{
		EventID:     "evt-" + evt.Info.ID,
		EventType:   contracts.EventMessageReceived,
		Timestamp:   evt.Info.Timestamp,
		ChatJID:     evt.Info.Chat.String(),
		SenderJID:   evt.Info.Sender.String(),
		SenderName:  evt.Info.PushName,
		IsGroup:     evt.Info.Chat.Server == types.GroupServer,
		IsFromMe:    evt.Info.IsFromMe,
		MessageID:   evt.Info.ID,
		ServerID:    strconv.FormatInt(int64(evt.Info.ServerID), 10),
		MessageType: contracts.WebhookMessageTypeUnknown,
		IsEdit:      evt.IsEdit,
	}
	if evt.Info.VerifiedName != nil {
		event.VerifiedName = evt.Info.VerifiedName.Details.GetVerifiedName()
	}

	if evt.Message == nil {
		return event
	}

	msg := evt.Message

	switch {
	case len(msg.GetConversation()) > 0:
		event.MessageType = contracts.WebhookMessageTypeText
		event.Text = msg.GetConversation()

	case msg.GetExtendedTextMessage() != nil:
		ext := msg.GetExtendedTextMessage()
		event.MessageType = contracts.WebhookMessageTypeText
		event.Text = ext.GetText()
		event.ReplyToID = ext.GetContextInfo().GetStanzaID()

	case msg.GetImageMessage() != nil:
		event.MessageType = contracts.WebhookMessageTypeImage
		img := msg.GetImageMessage()
		event.Text = img.GetCaption()
		event.Media = m.buildMedia(ctx, msg, "image")

	case msg.GetVideoMessage() != nil:
		event.MessageType = contracts.WebhookMessageTypeVideo
		vid := msg.GetVideoMessage()
		event.Text = vid.GetCaption()
		event.Media = m.buildMedia(ctx, msg, "video")

	case msg.GetAudioMessage() != nil:
		event.MessageType = contracts.WebhookMessageTypeAudio
		event.Media = m.buildMedia(ctx, msg, "audio")

	case msg.GetDocumentMessage() != nil:
		event.MessageType = contracts.WebhookMessageTypeDocument
		doc := msg.GetDocumentMessage()
		event.Text = doc.GetCaption()
		event.Media = m.buildMedia(ctx, msg, "document")

	case msg.GetStickerMessage() != nil:
		event.MessageType = contracts.WebhookMessageTypeSticker
		event.Media = m.buildMedia(ctx, msg, "sticker")

	case msg.GetButtonsResponseMessage() != nil:
		btn := msg.GetButtonsResponseMessage()
		event.MessageType = contracts.WebhookMessageTypeButtonReply
		event.ButtonReply = &contracts.WebhookButtonReply{
			SelectedID:   btn.GetSelectedButtonID(),
			SelectedText: btn.GetSelectedDisplayText(),
		}

	case msg.GetListResponseMessage() != nil:
		list := msg.GetListResponseMessage()
		event.MessageType = contracts.WebhookMessageTypeListReply
		event.ButtonReply = &contracts.WebhookButtonReply{
			SelectedID:   list.GetSingleSelectReply().GetSelectedRowID(),
			SelectedText: list.GetTitle(),
		}

	case msg.GetInteractiveResponseMessage() != nil:
		inter := msg.GetInteractiveResponseMessage()
		event.MessageType = contracts.WebhookMessageTypeButtonReply
		event.ButtonReply = &contracts.WebhookButtonReply{
			SelectedID:   inter.GetNativeFlowResponseMessage().GetName(),
			SelectedText: inter.GetBody().GetText(),
			ResponseJSON: inter.GetNativeFlowResponseMessage().GetParamsJSON(),
		}

	case msg.GetReactionMessage() != nil:
		reaction := msg.GetReactionMessage()
		event.MessageType = contracts.WebhookMessageTypeReaction
		event.Reaction = &contracts.WebhookReaction{
			Emoji:           reaction.GetText(),
			TargetMessageID: reaction.GetKey().GetID(),
			TargetSenderJID: reaction.GetKey().GetParticipant(),
			ByMe:            evt.Info.IsFromMe,
		}

	case msg.GetProtocolMessage() != nil && msg.GetProtocolMessage().GetType() == waE2E.ProtocolMessage_REVOKE:
		protocol := msg.GetProtocolMessage()
		event.MessageType = contracts.WebhookMessageTypeRevoke
		event.Revoke = &contracts.WebhookRevoke{
			TargetMessageID: protocol.GetKey().GetID(),
			TargetSenderJID: protocol.GetKey().GetParticipant(),
		}

	case msg.GetPollCreationMessage() != nil || msg.GetPollCreationMessageV2() != nil || msg.GetPollCreationMessageV3() != nil:
		event.MessageType = contracts.WebhookMessageTypePoll
		event.Poll = m.buildPollCreation(msg, evt)
		m.cachePollCreation(ctx, event.Poll, event.ChatJID)

	case msg.GetPollUpdateMessage() != nil:
		event.MessageType = contracts.WebhookMessageTypePollVote
		event.ReplyToID = msg.GetPollUpdateMessage().GetPollCreationMessageKey().GetID()
		event.PollVote = m.buildPollVote(ctx, evt)
	}

	if event.IsEdit && event.Text != "" {
		event.EditedText = event.Text
	}

	return event
}

// buildPollCreation extrai a enquete recebida (âncora do cache de hashes).
func (m *Manager) buildPollCreation(msg *waE2E.Message, evt *events.Message) *contracts.WebhookPollCreation {
	var name string
	var options []string
	var selectable int

	switch {
	case msg.GetPollCreationMessage() != nil:
		poll := msg.GetPollCreationMessage()
		name = poll.GetName()
		selectable = int(poll.GetSelectableOptionsCount())
		for _, option := range poll.GetOptions() {
			options = append(options, option.GetOptionName())
		}
	case msg.GetPollCreationMessageV2() != nil:
		poll := msg.GetPollCreationMessageV2()
		name = poll.GetName()
		selectable = int(poll.GetSelectableOptionsCount())
		for _, option := range poll.GetOptions() {
			options = append(options, option.GetOptionName())
		}
	case msg.GetPollCreationMessageV3() != nil:
		poll := msg.GetPollCreationMessageV3()
		name = poll.GetName()
		selectable = int(poll.GetSelectableOptionsCount())
		for _, option := range poll.GetOptions() {
			options = append(options, option.GetOptionName())
		}
	}

	return &contracts.WebhookPollCreation{
		PollMessageID:         evt.Info.ID,
		Question:              name,
		Options:               options,
		SelectableOptionCount: selectable,
		PollSenderJID:         evt.Info.Sender.String(),
	}
}

// buildMedia descreve a mídia e, quando habilitado, baixa e registra o arquivo.
func (m *Manager) buildMedia(ctx context.Context, msg *waE2E.Message, kind string) *contracts.WebhookMediaPayload {
	payload := &contracts.WebhookMediaPayload{}

	switch kind {
	case "image":
		img := msg.GetImageMessage()
		payload.MimeType = img.GetMimetype()
		payload.FileSize = int64(img.GetFileLength())
		payload.Width = int(img.GetWidth())
		payload.Height = int(img.GetHeight())
		payload.Caption = img.GetCaption()
	case "video":
		vid := msg.GetVideoMessage()
		payload.MimeType = vid.GetMimetype()
		payload.FileSize = int64(vid.GetFileLength())
		payload.Width = int(vid.GetWidth())
		payload.Height = int(vid.GetHeight())
		payload.Seconds = int(vid.GetSeconds())
		payload.Caption = vid.GetCaption()
		payload.IsAnimated = vid.GetGifPlayback()
	case "audio":
		audio := msg.GetAudioMessage()
		payload.MimeType = audio.GetMimetype()
		payload.FileSize = int64(audio.GetFileLength())
		payload.Seconds = int(audio.GetSeconds())
		payload.IsPTT = audio.GetPTT()
	case "document":
		doc := msg.GetDocumentMessage()
		payload.MimeType = doc.GetMimetype()
		payload.FileSize = int64(doc.GetFileLength())
		payload.Filename = doc.GetFileName()
		payload.Caption = doc.GetCaption()
	case "sticker":
		sticker := msg.GetStickerMessage()
		payload.MimeType = sticker.GetMimetype()
		payload.FileSize = int64(sticker.GetFileLength())
		payload.Width = int(sticker.GetWidth())
		payload.Height = int(sticker.GetHeight())
		payload.IsAnimated = sticker.GetIsAnimated()
	}

	if payload.MimeType == "" {
		payload.MimeType = "application/octet-stream"
	}

	if m.cfg == nil || !m.cfg.Webhook.AutoDownloadMedia || m.media == nil {
		return payload
	}

	data, err := m.client.DownloadAny(ctx, msg)
	if err != nil {
		m.client.Log.Warnf("falha ao baixar mídia: %v", err)
		return payload
	}

	ref, err := m.media.Save(ctx, data, payload.MimeType, payload.Filename)
	if err != nil {
		m.client.Log.Warnf("falha ao armazenar mídia: %v", err)
		return payload
	}

	payload.MediaID = ref.ID
	payload.DownloadURL = ref.DownloadURL
	payload.ExpiresAt = &ref.ExpiresAt

	if m.cfg.Webhook.IncludeBase64 {
		payload.Base64 = base64.StdEncoding.EncodeToString(data)
	}

	return payload
}

var _ = strings.TrimSpace

// publishGroupEvent publica o delta de um evento de grupo.
func (m *Manager) publishGroupEvent(evt *events.GroupInfo) {
	if m.publisher == nil || evt == nil {
		return
	}
	event := m.normalizeGroupEvent(evt)
	if event == nil {
		return
	}
	m.publisher.Publish(context.Background(), *event)
}

// normalizeGroupEvent converte *events.GroupInfo no envelope compartilhado.
func (m *Manager) normalizeGroupEvent(evt *events.GroupInfo) *contracts.WebhookInboundEvent {
	group := &contracts.WebhookGroupEvent{
		JID:     evt.JID.String(),
		Changes: []contracts.WebhookGroupChange{},
	}

	if evt.Sender != nil {
		group.SenderJID = evt.Sender.String()
	}

	if evt.Name != nil {
		value := evt.Name.Name
		group.Name = &value
		group.Changes = append(group.Changes, contracts.WebhookGroupChange{Type: contracts.GroupChangeSubject})
	}
	if evt.Topic != nil {
		value := evt.Topic.Topic
		group.Topic = &value
		group.Changes = append(group.Changes, contracts.WebhookGroupChange{Type: contracts.GroupChangeDescription})
	}
	if evt.Locked != nil {
		value := evt.Locked.IsLocked
		group.IsLocked = &value
		group.Changes = append(group.Changes, contracts.WebhookGroupChange{Type: contracts.GroupChangeLocked})
	}
	if evt.Announce != nil {
		value := evt.Announce.IsAnnounce
		group.IsAnnounce = &value
		group.Changes = append(group.Changes, contracts.WebhookGroupChange{Type: contracts.GroupChangeAnnounce})
	}
	if evt.Ephemeral != nil {
		group.Changes = append(group.Changes, contracts.WebhookGroupChange{Type: contracts.GroupChangeEphemeral})
	}
	if evt.Delete != nil && evt.Delete.Deleted {
		group.Changes = append(group.Changes, contracts.WebhookGroupChange{Type: contracts.GroupChangeDelete})
	}
	if evt.MembershipApprovalMode != nil {
		group.Changes = append(group.Changes, contracts.WebhookGroupChange{Type: contracts.GroupChangeMembershipApprovalMode})
	}
	if len(evt.Join) > 0 {
		group.Changes = append(group.Changes, contracts.WebhookGroupChange{Type: contracts.GroupChangeParticipantAdd, Participants: jidStrings(evt.Join)})
	}
	if len(evt.Leave) > 0 {
		group.Changes = append(group.Changes, contracts.WebhookGroupChange{Type: contracts.GroupChangeParticipantRemove, Participants: jidStrings(evt.Leave)})
	}
	if len(evt.Promote) > 0 {
		group.Changes = append(group.Changes, contracts.WebhookGroupChange{Type: contracts.GroupChangeParticipantPromote, Participants: jidStrings(evt.Promote)})
	}
	if len(evt.Demote) > 0 {
		group.Changes = append(group.Changes, contracts.WebhookGroupChange{Type: contracts.GroupChangeParticipantDemote, Participants: jidStrings(evt.Demote)})
	}
	if len(group.Changes) == 0 {
		group.Changes = append(group.Changes, contracts.WebhookGroupChange{Type: contracts.GroupChangeOther})
	}

	return &contracts.WebhookInboundEvent{
		EventID:   fmt.Sprintf("grp-%s-%d", evt.JID.String(), evt.Timestamp.UnixNano()),
		EventType: contracts.EventGroupUpdated,
		Timestamp: evt.Timestamp,
		ChatJID:   evt.JID.String(),
		SenderJID: group.SenderJID,
		IsGroup:   true,
		Group:     group,
	}
}

// jidStrings converte uma lista de JIDs em strings.
func jidStrings(jids []types.JID) []string {
	out := make([]string, 0, len(jids))
	for _, jid := range jids {
		out = append(out, jid.String())
	}
	return out
}
