package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gabedsam01/gabe-whats/shared/contracts"
)

// registerMessageActions publica interatividade e ações em mensagens.
func (s *Server) registerMessageActions() {
	s.registerSendButtons()
	s.registerSendList()
	s.registerSendPoll()
	s.registerReaction()
	s.registerEditMessage()
	s.registerRevokeMessage()
	s.registerMarkRead()
	s.registerPresence()
}

func (s *Server) registerSendButtons() {
	type input struct {
		Body *contracts.SendButtonsRequest
	}
	type output struct {
		Body *contracts.MessageResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/messages/buttons",
		OperationID: "sendButtons",
		Summary:     "Envia botões interativos (quick_reply, cta_url, cta_copy)",
		Tags:        []string{"Messages"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}
		resp, err := s.deps.WhatsApp.SendButtons(ctx, *in.Body)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: resp}, nil
	})
}

func (s *Server) registerSendList() {
	type input struct {
		Body *contracts.SendListRequest
	}
	type output struct {
		Body *contracts.MessageResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/messages/list",
		OperationID: "sendList",
		Summary:     "Envia mensagem com menu de lista selecionável",
		Tags:        []string{"Messages"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}
		resp, err := s.deps.WhatsApp.SendList(ctx, *in.Body)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: resp}, nil
	})
}

func (s *Server) registerSendPoll() {
	type input struct {
		Body *contracts.SendPollRequest
	}
	type output struct {
		Body *contracts.MessageResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/messages/poll",
		OperationID: "sendPoll",
		Summary:     "Envia enquete interativa nativa",
		Tags:        []string{"Messages"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}
		resp, err := s.deps.WhatsApp.SendPoll(ctx, *in.Body)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: resp}, nil
	})
}

func (s *Server) registerReaction() {
	type input struct {
		Body *contracts.ReactionRequest
	}
	type output struct {
		Body *contracts.MessageResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/messages/reaction",
		OperationID: "sendReaction",
		Summary:     "Envia ou remove reação emoji em uma mensagem",
		Tags:        []string{"Messages"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}
		resp, err := s.deps.WhatsApp.React(ctx, *in.Body)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: resp}, nil
	})
}

func (s *Server) registerEditMessage() {
	type input struct {
		ID   string `path:"id" doc:"ID da mensagem"`
		Body *contracts.EditMessageRequest
	}
	type output struct {
		Body *contracts.MessageResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPut,
		Path:        "/api/v1/messages/{id}",
		OperationID: "editMessage",
		Summary:     "Edita o texto de uma mensagem enviada",
		Tags:        []string{"Messages"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}
		resp, err := s.deps.WhatsApp.EditMessage(ctx, in.ID, *in.Body)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: resp}, nil
	})
}

func (s *Server) registerRevokeMessage() {
	type input struct {
		ID   string `path:"id" doc:"ID da mensagem"`
		Body *contracts.RevokeMessageRequest
	}
	type output struct {
		Body *contracts.MessageResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodDelete,
		Path:        "/api/v1/messages/{id}",
		OperationID: "revokeMessage",
		Summary:     "Revoga uma mensagem para todos",
		Tags:        []string{"Messages"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}
		resp, err := s.deps.WhatsApp.RevokeMessage(ctx, in.ID, *in.Body)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: resp}, nil
	})
}

func (s *Server) registerMarkRead() {
	type input struct {
		Body *contracts.MarkReadRequest
	}
	type output struct {
		Body *contracts.AckResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/messages/read",
		OperationID: "markRead",
		Summary:     "Marca mensagens como lidas",
		Tags:        []string{"Messages"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}
		resp, err := s.deps.WhatsApp.MarkRead(ctx, *in.Body)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: resp}, nil
	})
}

func (s *Server) registerPresence() {
	type input struct {
		Body *contracts.PresenceRequest
	}
	type output struct {
		Body *contracts.AckResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/messages/presence",
		OperationID: "sendPresence",
		Summary:     "Envia indicador de digitando ou gravando áudio",
		Tags:        []string{"Messages"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}
		resp, err := s.deps.WhatsApp.SendPresence(ctx, *in.Body)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: resp}, nil
	})
}
