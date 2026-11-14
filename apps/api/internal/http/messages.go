package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/whatsapp"
	"github.com/gabedsam01/gabe-whats/shared/contracts"
)

// registerMessages publica os endpoints de envio e histórico recente.
func (s *Server) registerMessages() {
	s.registerSendText()
	s.registerSendMedia()
	s.registerSendVoice()
	s.registerRecentMessages()
}

func (s *Server) registerSendText() {
	type input struct {
		Body *contracts.SendTextRequest
	}
	type output struct {
		Body *contracts.MessageResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/messages/text",
		OperationID: "sendText",
		Summary:     "Envia mensagem de texto (com suporte a reply e menções)",
		Tags:        []string{"Messages"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}
		if s.deps.WhatsApp == nil {
			return nil, newProblem(http.StatusServiceUnavailable, "whatsapp_unavailable", "cliente do WhatsApp não inicializado")
		}
		resp, err := s.deps.WhatsApp.SendText(ctx, *in.Body)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: resp}, nil
	})
}

func (s *Server) registerSendMedia() {
	type input struct {
		Body *contracts.SendMediaRequest
	}
	type output struct {
		Body *contracts.MessageResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/messages/media",
		OperationID: "sendMedia",
		Summary:     "Envia imagem, vídeo ou documento",
		Tags:        []string{"Messages"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}
		if s.deps.WhatsApp == nil {
			return nil, newProblem(http.StatusServiceUnavailable, "whatsapp_unavailable", "cliente do WhatsApp não inicializado")
		}
		resp, err := s.deps.WhatsApp.SendMedia(ctx, *in.Body)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: resp}, nil
	})
}

func (s *Server) registerSendVoice() {
	type input struct {
		Body *contracts.SendVoiceRequest
	}
	type output struct {
		Body *contracts.MessageResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/messages/voice",
		OperationID: "sendVoice",
		Summary:     "Envia áudio como nota de voz (PTT)",
		Tags:        []string{"Messages"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}
		if s.deps.WhatsApp == nil {
			return nil, newProblem(http.StatusServiceUnavailable, "whatsapp_unavailable", "cliente do WhatsApp não inicializado")
		}
		resp, err := s.deps.WhatsApp.SendVoice(ctx, *in.Body)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: resp}, nil
	})
}

func (s *Server) registerRecentMessages() {
	type input struct {
		Limit   int    `query:"limit" default:"50" minimum:"1" maximum:"200" doc:"Quantidade máxima de mensagens (padrão 50, máximo 200)"`
		ChatJID string `query:"chat_jid" doc:"Filtra por chat (opcional)"`
	}
	type output struct {
		Body *contracts.RecentMessagesResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/api/v1/messages/recent",
		OperationID: "listRecentMessages",
		Summary:     "Lista as mensagens recebidas mais recentes",
		Tags:        []string{"Messages"},
	}, func(_ context.Context, in *input) (*output, error) {
		result := &contracts.RecentMessagesResponse{Messages: []contracts.MessageSummary{}}
		if s.deps.Bus == nil {
			return &output{Body: result}, nil
		}

		events := s.deps.Bus.Recent().List(in.ChatJID, in.Limit)
		result.Messages = make([]contracts.MessageSummary, 0, len(events))
		for _, event := range events {
			result.Messages = append(result.Messages, contracts.MessageSummary{
				MessageID:   event.MessageID,
				ChatJID:     event.ChatJID,
				SenderJID:   event.SenderJID,
				SenderName:  event.SenderName,
				Timestamp:   event.Timestamp,
				MessageType: string(event.MessageType),
				Text:        event.Text,
				IsGroup:     event.IsGroup,
			})
		}
		return &output{Body: result}, nil
	})
}

// sendError traduz falhas de envio em Problem.
func sendError(err error) error {
	if errors.Is(err, whatsapp.ErrInvalidInput) {
		return newProblem(http.StatusUnprocessableEntity, "validation_failed", err.Error())
	}
	return newProblem(http.StatusBadGateway, "whatsapp_error", err.Error())
}
