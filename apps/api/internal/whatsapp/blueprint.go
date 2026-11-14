// Package whatsapp defines the architectural interface wrapping tulir/whatsmeow.
// O objetivo desta camada é isolar Protobufs (waE2E), JIDs, nós <biz> e sqlstore
// dos handlers HTTP e do contrato JSON compartilhado.
package whatsapp

import (
	"context"

	"github.com/gabedsam01/gabe-whats/shared/contracts"
)

// Service defines the full WhatsApp capabilities exposed to the REST layer.
type Service interface {
	// Sessão & Pareamento (client.go, qrchan.go, pair-code.go)
	Status(ctx context.Context) (*contracts.SessionStatus, error)
	GetQR(ctx context.Context) (*contracts.QRCodeResponse, error)
	PairPhone(ctx context.Context, phone string) (string, error)
	Reconnect(ctx context.Context) error
	Logout(ctx context.Context) error

	// Envio de Mensagens (send.go, upload.go, msgsecret.go)
	SendText(ctx context.Context, req contracts.SendTextRequest) (*contracts.MessageResponse, error)
	SendMedia(ctx context.Context, req contracts.SendMediaRequest) (*contracts.MessageResponse, error)
	SendVoicePTT(ctx context.Context, req contracts.SendVoiceRequest) (*contracts.MessageResponse, error)

	// Interatividade:
	// - SendButtons: usa waE2E.InteractiveMessage (NativeFlowMessage) + SendRequestExtra.AdditionalNodes (<biz>)
	// - SendList: usa waE2E.ListMessage + SendRequestExtra.AdditionalNodes (<biz>)
	// - SendPoll: usa cli.BuildPollCreation (100% nativo em contas pessoais)
	SendButtons(ctx context.Context, req contracts.SendButtonsRequest) (*contracts.MessageResponse, error)
	SendList(ctx context.Context, req contracts.SendListRequest) (*contracts.MessageResponse, error)
	SendPoll(ctx context.Context, req contracts.SendPollRequest) (*contracts.MessageResponse, error)

	// Ações em Mensagens (send.go, receipt.go, presence.go)
	React(ctx context.Context, req contracts.ReactionRequest) (*contracts.MessageResponse, error)
	Edit(ctx context.Context, messageID string, req contracts.EditMessageRequest) (*contracts.MessageResponse, error)
	Revoke(ctx context.Context, messageID string, chatJID string) (*contracts.MessageResponse, error)
	MarkRead(ctx context.Context, chatJID, senderJID string, messageIDs []string) error
	SendPresence(ctx context.Context, chatJID, state string) error

	// Gestão de Grupos (group.go)
	ListGroups(ctx context.Context) ([]contracts.GroupInfoResponse, error)
	GetGroup(ctx context.Context, groupJID string) (*contracts.GroupInfoResponse, error)
	CreateGroup(ctx context.Context, req contracts.CreateGroupRequest) (*contracts.GroupInfoResponse, error)
	UpdateGroupParticipants(ctx context.Context, groupJID string, req contracts.UpdateGroupParticipantsRequest) (*contracts.UpdateGroupParticipantsResponse, error)
	UpdateGroupSettings(ctx context.Context, groupJID string, req contracts.UpdateGroupSettingsRequest) error
	SetGroupPhoto(ctx context.Context, groupJID string, imageBytes []byte) (string, error)
	GetGroupInviteLink(ctx context.Context, groupJID string, reset bool) (string, error)
	JoinGroupWithLink(ctx context.Context, codeOrURL string) (string, error)
	LeaveGroup(ctx context.Context, groupJID string) error

	// Contatos (user.go)
	CheckOnWhatsApp(ctx context.Context, phones []string) (map[string]string, error)
}
