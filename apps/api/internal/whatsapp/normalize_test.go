package whatsapp

import (
	"context"
	"testing"
	"time"

	"github.com/gabedsam01/gabe-whats/shared/contracts"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// testManager cria um Manager mínimo: nenhum caso abaixo toca em rede, mídia ou
// banco (cfg nil desliga o download e o client só é usado para log).
func testManager() *Manager {
	return &Manager{client: &whatsmeow.Client{Log: waLog.Noop}}
}

// testEvent monta um evento recebido com a mensagem informada.
func testEvent(msg *waE2E.Message) *events.Message {
	jid := types.NewJID("5511999999999", types.DefaultUserServer)
	return &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: jid, Sender: jid},
			ID:            "3EB0TESTE",
			Timestamp:     time.Unix(0, 0),
		},
		Message: msg,
	}
}

func TestNormalizeMapeiaTipos(t *testing.T) {
	manager := testManager()
	ctx := context.Background()

	t.Run("texto", func(t *testing.T) {
		event := manager.normalizeMessage(ctx, testEvent(&waE2E.Message{Conversation: proto.String("oi")}))
		if event == nil || event.MessageType != contracts.WebhookMessageTypeText || event.Text != "oi" {
			t.Fatalf("evento inesperado: %+v", event)
		}
	})

	t.Run("reacao", func(t *testing.T) {
		msg := &waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{
			Text: proto.String("👍"),
			Key:  &waCommon.MessageKey{ID: proto.String("ALVO123")},
		}}
		event := manager.normalizeMessage(ctx, testEvent(msg))
		if event == nil || event.MessageType != contracts.WebhookMessageTypeReaction {
			t.Fatalf("tipo inesperado: %+v", event)
		}
		if event.Reaction == nil || event.Reaction.Emoji != "👍" || event.Reaction.TargetMessageID != "ALVO123" {
			t.Fatalf("reação inesperada: %+v", event.Reaction)
		}
	})

	t.Run("localizacao ao vivo", func(t *testing.T) {
		msg := &waE2E.Message{LiveLocationMessage: &waE2E.LiveLocationMessage{
			Caption: proto.String("a caminho"),
		}}
		event := manager.normalizeMessage(ctx, testEvent(msg))
		if event == nil || event.MessageType != contracts.WebhookMessageTypeLocation || event.Text != "a caminho" {
			t.Fatalf("evento inesperado: %+v", event)
		}
	})

	t.Run("video nota (ptv)", func(t *testing.T) {
		msg := &waE2E.Message{PtvMessage: &waE2E.VideoMessage{
			Mimetype:   proto.String("video/mp4"),
			FileLength: proto.Uint64(2048),
			Width:      proto.Uint32(320),
			Height:     proto.Uint32(240),
			Seconds:    proto.Uint32(4),
		}}
		event := manager.normalizeMessage(ctx, testEvent(msg))
		if event == nil || event.MessageType != contracts.WebhookMessageTypeVideo {
			t.Fatalf("tipo inesperado: %+v", event)
		}
		if event.Media == nil || event.Media.MimeType != "video/mp4" || event.Media.FileSize != 2048 {
			t.Fatalf("mídia inesperada: %+v", event.Media)
		}
	})

	t.Run("protocolo desconhecido e descartado", func(t *testing.T) {
		msg := &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
			Type: waE2E.ProtocolMessage_APP_STATE_SYNC_KEY_SHARE.Enum(),
		}}
		if event := manager.normalizeMessage(ctx, testEvent(msg)); event != nil {
			t.Fatalf("esperava descarte, obtive %+v", event)
		}
	})
}
