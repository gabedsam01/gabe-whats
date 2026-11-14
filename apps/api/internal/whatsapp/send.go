package whatsapp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gabedsam01/gabe-whats/shared/contracts"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// ErrInvalidInput marca falhas de validação do payload de envio.
var ErrInvalidInput = errors.New("entrada inválida")

// invalidInput embrulha um erro de validação.
func invalidInput(err error) error {
	return fmt.Errorf("%w: %v", ErrInvalidInput, err)
}

// maxSourceBytes limita o tamanho aceito para envio de mídia (32 MiB).
const maxSourceBytes = 32 << 20

// parseRecipient aceita número com DDI ou JID completo.
func parseRecipient(raw string) (types.JID, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return types.JID{}, errors.New("destinatário vazio")
	}
	if strings.Contains(value, "@") {
		jid, err := types.ParseJID(value)
		if err != nil {
			return types.JID{}, fmt.Errorf("JID inválido %q: %w", value, err)
		}
		return jid, nil
	}

	digits := onlyDigits(value)
	if len(digits) < 10 || len(digits) > 15 {
		return types.JID{}, fmt.Errorf("número inválido %q: use DDI + número (10 a 15 dígitos)", raw)
	}
	return types.NewJID(digits, types.DefaultUserServer), nil
}

// onlyDigits remove qualquer caractere que não seja dígito.
func onlyDigits(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SendText envia uma mensagem de texto, opcionalmente citando e mencionando.
func (m *Manager) SendText(ctx context.Context, req contracts.SendTextRequest) (*contracts.MessageResponse, error) {
	to, err := parseRecipient(req.To)
	if err != nil {
		return nil, invalidInput(err)
	}
	if strings.TrimSpace(req.Text) == "" {
		return nil, invalidInput(errors.New("texto vazio"))
	}

	if req.SimulateTypingMs > 0 {
		m.simulateTyping(ctx, to, time.Duration(req.SimulateTypingMs)*time.Millisecond)
	}

	msg := &waE2E.Message{}
	mentions := make([]string, 0, len(req.Mentions))
	for _, mention := range req.Mentions {
		jid, err := parseRecipient(mention)
		if err != nil {
			return nil, invalidInput(fmt.Errorf("menção inválida %q: %w", mention, err))
		}
		mentions = append(mentions, jid.String())
	}

	if req.ReplyToID == "" && len(mentions) == 0 {
		msg.Conversation = proto.String(req.Text)
	} else {
		contextInfo := &waE2E.ContextInfo{}
		if req.ReplyToID != "" {
			contextInfo.StanzaID = proto.String(req.ReplyToID)
			contextInfo.Participant = proto.String(to.String())
		}
		if len(mentions) > 0 {
			contextInfo.MentionedJID = mentions
		}
		msg.ExtendedTextMessage = &waE2E.ExtendedTextMessage{
			Text:        proto.String(req.Text),
			ContextInfo: contextInfo,
		}
	}

	resp, err := m.client.SendMessage(ctx, to, msg)
	if err != nil {
		return nil, fmt.Errorf("enviar texto: %w", err)
	}
	return messageResponse(resp, to, "sent"), nil
}

// SendMedia envia imagem, vídeo ou documento a partir de URL, caminho ou base64.
func (m *Manager) SendMedia(ctx context.Context, req contracts.SendMediaRequest) (*contracts.MessageResponse, error) {
	to, err := parseRecipient(req.To)
	if err != nil {
		return nil, invalidInput(err)
	}

	data, mimeType, filename, err := loadSource(ctx, req.Source, req.MimeType, req.Filename)
	if err != nil {
		return nil, invalidInput(err)
	}

	var kind whatsmeow.MediaType
	switch strings.ToLower(req.MediaType) {
	case "image":
		kind = whatsmeow.MediaImage
	case "video":
		kind = whatsmeow.MediaVideo
	case "document":
		kind = whatsmeow.MediaDocument
	default:
		return nil, invalidInput(fmt.Errorf("media_type inválido %q: use image, video ou document", req.MediaType))
	}

	upload, err := m.client.Upload(ctx, data, kind)
	if err != nil {
		return nil, fmt.Errorf("upload de mídia: %w", err)
	}

	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}

	msg := &waE2E.Message{}
	switch kind {
	case whatsmeow.MediaImage:
		msg.ImageMessage = &waE2E.ImageMessage{
			URL:           proto.String(upload.URL),
			DirectPath:    proto.String(upload.DirectPath),
			MediaKey:      upload.MediaKey,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    proto.Uint64(upload.FileLength),
			Mimetype:      proto.String(mimeType),
			Caption:       proto.String(req.Caption),
		}
	case whatsmeow.MediaVideo:
		msg.VideoMessage = &waE2E.VideoMessage{
			URL:           proto.String(upload.URL),
			DirectPath:    proto.String(upload.DirectPath),
			MediaKey:      upload.MediaKey,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    proto.Uint64(upload.FileLength),
			Mimetype:      proto.String(mimeType),
			Caption:       proto.String(req.Caption),
		}
	default:
		msg.DocumentMessage = &waE2E.DocumentMessage{
			URL:           proto.String(upload.URL),
			DirectPath:    proto.String(upload.DirectPath),
			MediaKey:      upload.MediaKey,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    proto.Uint64(upload.FileLength),
			Mimetype:      proto.String(mimeType),
			FileName:      proto.String(filename),
			Caption:       proto.String(req.Caption),
		}
	}

	if req.ReplyToID != "" {
		applyReply(msg, req.ReplyToID, to)
	}

	resp, err := m.client.SendMessage(ctx, to, msg)
	if err != nil {
		return nil, fmt.Errorf("enviar mídia: %w", err)
	}
	return messageResponse(resp, to, "sent"), nil
}

// SendVoice envia áudio como nota de voz (PTT).
func (m *Manager) SendVoice(ctx context.Context, req contracts.SendVoiceRequest) (*contracts.MessageResponse, error) {
	to, err := parseRecipient(req.To)
	if err != nil {
		return nil, invalidInput(err)
	}

	data, mimeType, _, err := loadSource(ctx, req.Source, "", "")
	if err != nil {
		return nil, invalidInput(err)
	}
	if mimeType == "" {
		mimeType = "audio/ogg; codecs=opus"
	}

	upload, err := m.client.Upload(ctx, data, whatsmeow.MediaAudio)
	if err != nil {
		return nil, fmt.Errorf("upload de áudio: %w", err)
	}

	msg := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{
			URL:           proto.String(upload.URL),
			DirectPath:    proto.String(upload.DirectPath),
			MediaKey:      upload.MediaKey,
			FileEncSHA256: upload.FileEncSHA256,
			FileSHA256:    upload.FileSHA256,
			FileLength:    proto.Uint64(upload.FileLength),
			Mimetype:      proto.String(mimeType),
			PTT:           proto.Bool(true),
		},
	}
	if req.ReplyToID != "" {
		applyReply(msg, req.ReplyToID, to)
	}

	resp, err := m.client.SendMessage(ctx, to, msg)
	if err != nil {
		return nil, fmt.Errorf("enviar áudio: %w", err)
	}
	return messageResponse(resp, to, "sent"), nil
}

// applyReply anexa o contexto de citação à mensagem.
func applyReply(msg *waE2E.Message, replyTo string, to types.JID) {
	contextInfo := &waE2E.ContextInfo{
		StanzaID:    proto.String(replyTo),
		Participant: proto.String(to.String()),
	}
	switch {
	case msg.ImageMessage != nil:
		msg.ImageMessage.ContextInfo = contextInfo
	case msg.VideoMessage != nil:
		msg.VideoMessage.ContextInfo = contextInfo
	case msg.DocumentMessage != nil:
		msg.DocumentMessage.ContextInfo = contextInfo
	case msg.AudioMessage != nil:
		msg.AudioMessage.ContextInfo = contextInfo
	}
}

// simulateTyping publica "digitando..." e encerra após a duração pedida.
func (m *Manager) simulateTyping(ctx context.Context, to types.JID, duration time.Duration) {
	if err := m.client.SendChatPresence(ctx, to, types.ChatPresenceComposing, types.ChatPresenceMediaText); err != nil {
		return
	}
	select {
	case <-time.After(duration):
	case <-ctx.Done():
	}
	_ = m.client.SendChatPresence(ctx, to, types.ChatPresencePaused, types.ChatPresenceMediaText)
}

// messageResponse converte a resposta do whatsmeow no contrato compartilhado.
func messageResponse(resp whatsmeow.SendResponse, to types.JID, status string) *contracts.MessageResponse {
	return &contracts.MessageResponse{
		MessageID: resp.ID,
		ChatJID:   to.String(),
		Timestamp: resp.Timestamp,
		Status:    status,
	}
}

// loadSource resolve URL, data URI, prefixo base64: ou caminho local.
func loadSource(ctx context.Context, source, mimeHint, nameHint string) ([]byte, string, string, error) {
	value := strings.TrimSpace(source)
	if value == "" {
		return nil, "", "", errors.New("source vazio")
	}

	switch {
	case strings.HasPrefix(value, "data:"):
		header, payload, ok := strings.Cut(value, ",")
		if !ok {
			return nil, "", "", errors.New("data URI inválido")
		}
		mimeType := strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64"), ";")
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, "", "", fmt.Errorf("base64 inválido: %w", err)
		}
		return data, firstNonEmpty(mimeHint, mimeType), nameHint, nil

	case strings.HasPrefix(value, "base64:"):
		data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "base64:"))
		if err != nil {
			return nil, "", "", fmt.Errorf("base64 inválido: %w", err)
		}
		return data, mimeHint, nameHint, nil

	case strings.HasPrefix(value, "http://"), strings.HasPrefix(value, "https://"):
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, value, nil)
		if err != nil {
			return nil, "", "", err
		}
		client := &http.Client{Timeout: 30 * time.Second}
		response, err := client.Do(request)
		if err != nil {
			return nil, "", "", fmt.Errorf("baixar mídia: %w", err)
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode < 200 || response.StatusCode > 299 {
			return nil, "", "", fmt.Errorf("baixar mídia: status %d", response.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, maxSourceBytes+1))
		if err != nil {
			return nil, "", "", fmt.Errorf("ler mídia: %w", err)
		}
		if len(data) > maxSourceBytes {
			return nil, "", "", errors.New("mídia excede 32 MiB")
		}
		filename := firstNonEmpty(nameHint, path.Base(request.URL.Path))
		return data, firstNonEmpty(mimeHint, response.Header.Get("Content-Type")), filename, nil
	}

	info, err := os.Stat(value)
	if err != nil {
		return nil, "", "", fmt.Errorf("abrir arquivo: %w", err)
	}
	if info.Size() > maxSourceBytes {
		return nil, "", "", errors.New("arquivo excede 32 MiB")
	}
	data, err := os.ReadFile(value)
	if err != nil {
		return nil, "", "", fmt.Errorf("ler arquivo: %w", err)
	}
	return data, mimeHint, firstNonEmpty(nameHint, info.Name()), nil
}

// firstNonEmpty devolve o primeiro valor não vazio.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
