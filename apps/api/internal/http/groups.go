package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/whatsapp"
	"github.com/gabedsam01/gabe-whats/shared/contracts"
	"github.com/go-chi/chi/v5"
)

// maxPhotoBytes limita o upload de foto de grupo (8 MiB).
const maxPhotoBytes = 8 << 20

// registerGroups publica os endpoints de gestão de grupos.
func (s *Server) registerGroups() {
	s.registerListGroups()
	s.registerCreateGroup()
	s.registerGetGroup()
	s.registerUpdateGroupSettings()
	s.registerUpdateGroupParticipants()
	s.registerGroupInviteLink()
	s.registerJoinGroup()
	s.registerLeaveGroup()
	s.registerGroupPhoto()
}

func (s *Server) registerListGroups() {
	type output struct {
		Body *contracts.GroupsResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/api/v1/groups",
		OperationID: "listGroups",
		Summary:     "Lista todos os grupos participados",
		Tags:        []string{"Groups"},
	}, func(ctx context.Context, _ *struct{}) (*output, error) {
		groups, err := s.deps.WhatsApp.ListGroups(ctx)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: groups}, nil
	})
}

func (s *Server) registerCreateGroup() {
	type input struct {
		Body *contracts.CreateGroupRequest
	}
	type output struct {
		Body *contracts.GroupInfoResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/groups",
		OperationID: "createGroup",
		Summary:     "Cria um novo grupo",
		Tags:        []string{"Groups"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}
		group, err := s.deps.WhatsApp.CreateGroup(ctx, *in.Body)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: group}, nil
	})
}

func (s *Server) registerGetGroup() {
	type input struct {
		JID string `path:"jid" doc:"JID do grupo"`
	}
	type output struct {
		Body *contracts.GroupInfoResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/api/v1/groups/{jid}",
		OperationID: "getGroup",
		Summary:     "Obtém detalhes e participantes de um grupo",
		Tags:        []string{"Groups"},
	}, func(ctx context.Context, in *input) (*output, error) {
		group, err := s.deps.WhatsApp.GetGroup(ctx, in.JID)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: group}, nil
	})
}

func (s *Server) registerUpdateGroupSettings() {
	type input struct {
		JID  string `path:"jid" doc:"JID do grupo"`
		Body *contracts.UpdateGroupSettingsRequest
	}
	type output struct {
		Body *contracts.AckResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPatch,
		Path:        "/api/v1/groups/{jid}",
		OperationID: "updateGroupSettings",
		Summary:     "Atualiza nome, descrição, anúncio ou trava do grupo",
		Tags:        []string{"Groups"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}
		if err := s.deps.WhatsApp.UpdateGroupSettings(ctx, in.JID, *in.Body); err != nil {
			return nil, sendError(err)
		}
		return &output{Body: &contracts.AckResponse{OK: true, Detail: "configurações atualizadas"}}, nil
	})
}

func (s *Server) registerUpdateGroupParticipants() {
	type input struct {
		JID  string `path:"jid" doc:"JID do grupo"`
		Body *contracts.UpdateGroupParticipantsRequest
	}
	type output struct {
		Body *contracts.UpdateGroupParticipantsResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/groups/{jid}/participants",
		OperationID: "updateGroupParticipants",
		Summary:     "Adiciona, remove, promove ou rebaixa participantes",
		Tags:        []string{"Groups"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}
		result, err := s.deps.WhatsApp.UpdateGroupParticipants(ctx, in.JID, *in.Body)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: result}, nil
	})
}

func (s *Server) registerGroupInviteLink() {
	type input struct {
		JID   string `path:"jid" doc:"JID do grupo"`
		Reset bool   `query:"reset" doc:"Se true, revoga o link atual e gera um novo"`
	}
	type output struct {
		Body *contracts.InviteLinkResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/api/v1/groups/{jid}/invite-link",
		OperationID: "getGroupInviteLink",
		Summary:     "Obtém ou reseta o link de convite do grupo",
		Tags:        []string{"Groups"},
	}, func(ctx context.Context, in *input) (*output, error) {
		link, err := s.deps.WhatsApp.GetGroupInviteLink(ctx, in.JID, in.Reset)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: link}, nil
	})
}

func (s *Server) registerJoinGroup() {
	type input struct {
		Body *contracts.JoinGroupRequest
	}
	type output struct {
		Body *contracts.JoinGroupResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/groups/join",
		OperationID: "joinGroup",
		Summary:     "Entra em um grupo via código ou link de convite",
		Tags:        []string{"Groups"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}
		result, err := s.deps.WhatsApp.JoinGroupWithLink(ctx, in.Body.Invite)
		if err != nil {
			return nil, sendError(err)
		}
		return &output{Body: result}, nil
	})
}

func (s *Server) registerLeaveGroup() {
	type input struct {
		JID string `path:"jid" doc:"JID do grupo"`
	}
	type output struct {
		Body *contracts.AckResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/groups/{jid}/leave",
		OperationID: "leaveGroup",
		Summary:     "Sai de um grupo",
		Tags:        []string{"Groups"},
	}, func(ctx context.Context, in *input) (*output, error) {
		if err := s.deps.WhatsApp.LeaveGroup(ctx, in.JID); err != nil {
			return nil, sendError(err)
		}
		return &output{Body: &contracts.AckResponse{OK: true, Detail: "saída solicitada"}}, nil
	})
}

// registerGroupPhoto usa chi puro: multipart não passa pelo binding do Huma.
func (s *Server) registerGroupPhoto() {
	s.router.Put("/api/v1/groups/{jid}/photo", s.handleGroupPhoto)
}

func (s *Server) handleGroupPhoto(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxPhotoBytes); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "corpo multipart inválido")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validation_failed", "parte file ausente no formulário")
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, maxPhotoBytes+1))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "não foi possível ler o arquivo")
		return
	}
	if len(data) > maxPhotoBytes {
		writeProblem(w, http.StatusRequestEntityTooLarge, "payload_too_large", "arquivo excede 8 MiB")
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}

	result, err := s.deps.WhatsApp.SetGroupPhoto(r.Context(), chi.URLParam(r, "jid"), data, mimeType, r.FormValue("caption"))
	if err != nil {
		if errors.Is(err, whatsapp.ErrInvalidInput) {
			writeProblem(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
			return
		}
		writeProblem(w, http.StatusBadGateway, "whatsapp_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(result)
}
