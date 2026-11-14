// Package apiclient fala com a API REST do apps/api. O servidor MCP é
// stateless: toda operação passa por HTTP, nunca por whatsmeow nem pelo SQLite.
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gabedsam01/gabe-whats/shared/contracts"
)

// Client é o cliente HTTP da API.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New cria o cliente com a URL base e a chave da API.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// APIError representa uma resposta de erro da API (RFC 9457).
type APIError struct {
	Status  int
	Code    string
	Detail  string
	Problem contracts.Problem
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("API respondeu %d (%s): %s", e.Status, e.Code, e.Detail)
	}
	return fmt.Sprintf("API respondeu %d: %s", e.Status, e.Detail)
}

// do executa a requisição e decodifica a resposta em out.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("serializar requisição: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("montar requisição: %w", err)
	}
	if c.apiKey != "" {
		request.Header.Set("X-API-Key", c.apiKey)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Accept", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("chamar a API: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("ler resposta: %w", err)
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		apiErr := &APIError{Status: response.StatusCode, Detail: strings.TrimSpace(string(payload))}
		var problem contracts.Problem
		if jsonErr := json.Unmarshal(payload, &problem); jsonErr == nil {
			apiErr.Problem = problem
			apiErr.Code = problem.Code
			if problem.Detail != "" {
				apiErr.Detail = problem.Detail
			}
		}
		return apiErr
	}

	if out == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decodificar resposta: %w", err)
	}
	return nil
}

// Status devolve o estado da sessão.
func (c *Client) Status(ctx context.Context) (*contracts.SessionStatus, error) {
	var out contracts.SessionStatus
	if err := c.do(ctx, http.MethodGet, "/api/v1/session/status", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// QRCode devolve o QR de pareamento atual.
func (c *Client) QRCode(ctx context.Context) (*contracts.QRCodeResponse, error) {
	var out contracts.QRCodeResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/session/qr", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PairCode gera o código de pareamento por telefone.
func (c *Client) PairCode(ctx context.Context, req contracts.PairCodeRequest) (*contracts.PairCodeResponse, error) {
	var out contracts.PairCodeResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/session/pair-code", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CheckNumbers verifica quais números têm WhatsApp.
func (c *Client) CheckNumbers(ctx context.Context, req contracts.CheckNumbersRequest) (*contracts.CheckNumbersResponse, error) {
	var out contracts.CheckNumbersResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/contacts/check", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendText envia texto.
func (c *Client) SendText(ctx context.Context, req contracts.SendTextRequest) (*contracts.MessageResponse, error) {
	var out contracts.MessageResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/messages/text", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendMedia envia mídia.
func (c *Client) SendMedia(ctx context.Context, req contracts.SendMediaRequest) (*contracts.MessageResponse, error) {
	var out contracts.MessageResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/messages/media", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendVoice envia nota de voz.
func (c *Client) SendVoice(ctx context.Context, req contracts.SendVoiceRequest) (*contracts.MessageResponse, error) {
	var out contracts.MessageResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/messages/voice", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendButtons envia botões (pode ser recusado pelo WhatsApp em contas pessoais).
func (c *Client) SendButtons(ctx context.Context, req contracts.SendButtonsRequest) (*contracts.MessageResponse, error) {
	var out contracts.MessageResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/messages/buttons", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendList envia lista.
func (c *Client) SendList(ctx context.Context, req contracts.SendListRequest) (*contracts.MessageResponse, error) {
	var out contracts.MessageResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/messages/list", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendPoll envia enquete.
func (c *Client) SendPoll(ctx context.Context, req contracts.SendPollRequest) (*contracts.MessageResponse, error) {
	var out contracts.MessageResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/messages/poll", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// React envia ou remove reação.
func (c *Client) React(ctx context.Context, req contracts.ReactionRequest) (*contracts.MessageResponse, error) {
	var out contracts.MessageResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/messages/reaction", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EditMessage edita o texto de uma mensagem.
func (c *Client) EditMessage(ctx context.Context, messageID string, req contracts.EditMessageRequest) (*contracts.MessageResponse, error) {
	var out contracts.MessageResponse
	path := "/api/v1/messages/" + url.PathEscape(messageID)
	if err := c.do(ctx, http.MethodPut, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RevokeMessage apaga uma mensagem para todos.
func (c *Client) RevokeMessage(ctx context.Context, messageID string, req contracts.RevokeMessageRequest) (*contracts.MessageResponse, error) {
	var out contracts.MessageResponse
	path := "/api/v1/messages/" + url.PathEscape(messageID)
	if err := c.do(ctx, http.MethodDelete, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MarkRead marca mensagens como lidas.
func (c *Client) MarkRead(ctx context.Context, req contracts.MarkReadRequest) (*contracts.AckResponse, error) {
	var out contracts.AckResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/messages/read", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RecentMessages lista o histórico recente.
func (c *Client) RecentMessages(ctx context.Context, limit int, chatJID string) (*contracts.RecentMessagesResponse, error) {
	query := url.Values{}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if chatJID != "" {
		query.Set("chat_jid", chatJID)
	}
	path := "/api/v1/messages/recent"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var out contracts.RecentMessagesResponse
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListGroups lista os grupos.
func (c *Client) ListGroups(ctx context.Context) (*contracts.GroupsResponse, error) {
	var out contracts.GroupsResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/groups", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetGroup devolve um grupo.
func (c *Client) GetGroup(ctx context.Context, groupJID string) (*contracts.GroupInfoResponse, error) {
	var out contracts.GroupInfoResponse
	if err := c.do(ctx, http.MethodGet, "/api/v1/groups/"+url.PathEscape(groupJID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateGroup cria um grupo.
func (c *Client) CreateGroup(ctx context.Context, req contracts.CreateGroupRequest) (*contracts.GroupInfoResponse, error) {
	var out contracts.GroupInfoResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/groups", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateGroupParticipants altera participantes.
func (c *Client) UpdateGroupParticipants(ctx context.Context, groupJID string, req contracts.UpdateGroupParticipantsRequest) (*contracts.UpdateGroupParticipantsResponse, error) {
	var out contracts.UpdateGroupParticipantsResponse
	path := "/api/v1/groups/" + url.PathEscape(groupJID) + "/participants"
	if err := c.do(ctx, http.MethodPost, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateGroupSettings altera metadados do grupo.
func (c *Client) UpdateGroupSettings(ctx context.Context, groupJID string, req contracts.UpdateGroupSettingsRequest) (*contracts.AckResponse, error) {
	var out contracts.AckResponse
	if err := c.do(ctx, http.MethodPatch, "/api/v1/groups/"+url.PathEscape(groupJID), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GroupInviteLink devolve (ou reseta) o link de convite.
func (c *Client) GroupInviteLink(ctx context.Context, groupJID string, reset bool) (*contracts.InviteLinkResponse, error) {
	path := "/api/v1/groups/" + url.PathEscape(groupJID) + "/invite-link"
	if reset {
		path += "?reset=true"
	}
	var out contracts.InviteLinkResponse
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// JoinGroup entra por link/código de convite.
func (c *Client) JoinGroup(ctx context.Context, invite string) (*contracts.JoinGroupResponse, error) {
	var out contracts.JoinGroupResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/groups/join", contracts.JoinGroupRequest{Invite: invite}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// LeaveGroup sai do grupo.
func (c *Client) LeaveGroup(ctx context.Context, groupJID string) (*contracts.AckResponse, error) {
	var out contracts.AckResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/groups/"+url.PathEscape(groupJID)+"/leave", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
