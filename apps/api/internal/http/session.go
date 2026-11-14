package httpapi

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/whatsapp"
	"github.com/gabedsam01/gabe-whats/shared/contracts"
)

// phonePattern aceita apenas dígitos com DDI: 10 a 15 caracteres.
var phonePattern = regexp.MustCompile("^[0-9]{10,15}$")

// registerSession publica os endpoints de sessão e pareamento.
func (s *Server) registerSession() {
	s.registerSessionStatus()
	s.registerSessionQR()
	s.registerSessionPairCode()
	s.registerSessionReconnect()
	s.registerSessionLogout()
}

func (s *Server) registerSessionStatus() {
	type output struct {
		Body *contracts.SessionStatus
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/api/v1/session/status",
		OperationID: "sessionStatus",
		Summary:     "Status da sessão do WhatsApp",
		Tags:        []string{"Session"},
	}, func(ctx context.Context, _ *struct{}) (*output, error) {
		status, err := s.wa.Status(ctx)
		if err != nil {
			return nil, newProblem(http.StatusInternalServerError, "internal_error", "não foi possível consultar a sessão")
		}
		return &output{Body: status}, nil
	})
}

func (s *Server) registerSessionQR() {
	type output struct {
		Body *contracts.QRCodeResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/api/v1/session/qr",
		OperationID: "sessionQRCode",
		Summary:     "Obtém o QR Code atual para pareamento",
		Tags:        []string{"Session"},
	}, func(ctx context.Context, _ *struct{}) (*output, error) {
		if err := s.wa.StartPairing(ctx); err != nil {
			return nil, sessionError(err)
		}
		qr, err := s.wa.GetQR(ctx)
		if err != nil {
			return nil, sessionError(err)
		}
		return &output{Body: qr}, nil
	})
}

func (s *Server) registerSessionPairCode() {
	type input struct {
		Body *contracts.PairCodeRequest
	}
	type output struct {
		Body *contracts.PairCodeResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/session/pair-code",
		OperationID: "sessionPairCode",
		Summary:     "Gera código de pareamento de 8 dígitos por telefone",
		Tags:        []string{"Session"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}

		phone := normalizePhone(in.Body.Phone)
		if !phonePattern.MatchString(phone) {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed",
				"número inválido: informe apenas dígitos com DDI (10 a 15 caracteres)",
				fieldError("phone", "esperado número com DDI, apenas dígitos"))
		}

		code, err := s.wa.PairPhone(ctx, phone)
		if err != nil {
			return nil, sessionError(err)
		}
		return &output{Body: code}, nil
	})
}

func (s *Server) registerSessionReconnect() {
	type output struct {
		Body *contracts.AckResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/session/reconnect",
		OperationID: "sessionReconnect",
		Summary:     "Reconecta o cliente whatsmeow",
		Tags:        []string{"Session"},
	}, func(ctx context.Context, _ *struct{}) (*output, error) {
		if err := s.wa.Reconnect(ctx); err != nil {
			return nil, sessionError(err)
		}
		return &output{Body: &contracts.AckResponse{OK: true, Detail: "reconexão solicitada"}}, nil
	})
}

func (s *Server) registerSessionLogout() {
	type output struct {
		Body *contracts.AckResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/session/logout",
		OperationID: "sessionLogout",
		Summary:     "Faz logout e remove vínculo do dispositivo",
		Tags:        []string{"Session"},
	}, func(ctx context.Context, _ *struct{}) (*output, error) {
		if err := s.wa.Logout(ctx); err != nil {
			return nil, sessionError(err)
		}
		return &output{Body: &contracts.AckResponse{OK: true, Detail: "sessão encerrada"}}, nil
	})
}

// sessionError traduz erros do serviço de WhatsApp em Problem apropriado.
func sessionError(err error) error {
	switch {
	case errors.Is(err, whatsapp.ErrAlreadyPaired):
		return newProblem(http.StatusConflict, "already_paired", "o dispositivo já está pareado e conectado")
	case errors.Is(err, whatsapp.ErrPairingInactive):
		return newProblem(http.StatusConflict, "pairing_not_started", "inicie o pareamento consultando GET /api/v1/session/qr")
	case errors.Is(err, whatsapp.ErrPairingTimeout):
		return newProblem(http.StatusGatewayTimeout, "pairing_timeout", "tempo esgotado aguardando o QR Code")
	default:
		return newProblem(http.StatusInternalServerError, "whatsapp_error", err.Error())
	}
}

// normalizePhone remove tudo que não for dígito.
func normalizePhone(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
