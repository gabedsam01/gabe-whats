// interactive.go implementa as mensagens interativas da fase S4: botões
// NativeFlow e listas expansíveis. Ambos os formatos exigem a injeção do nó
// <biz> no stanza (SendRequestExtra.AdditionalNodes), requisito do protocolo
// Multi-Device para que o cliente oficial renderize a interatividade.

package whatsapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gabedsam01/gabe-whats/shared/contracts"
	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// SendButtons envia uma mensagem interativa com até 3 botões NativeFlow
// (quick_reply, cta_url ou cta_copy). O corpo é obrigatório; header e footer
// são opcionais. O clique do destinatário chega no webhook como resposta de
// native flow, carregando o ID informado no botão.
func (m *Manager) SendButtons(ctx context.Context, req contracts.SendButtonsRequest) (*contracts.MessageResponse, error) {
	to, err := parseRecipient(req.To)
	if err != nil {
		return nil, invalidInput(err)
	}
	if strings.TrimSpace(req.Body) == "" {
		return nil, invalidInput(errors.New("body é obrigatório"))
	}
	if len(req.Buttons) < 1 || len(req.Buttons) > 3 {
		return nil, invalidInput(fmt.Errorf("a mensagem aceita de 1 a 3 botões, recebido %d", len(req.Buttons)))
	}

	buttons := make([]*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton, 0, len(req.Buttons))
	for index, button := range req.Buttons {
		params, err := buildButtonParams(button)
		if err != nil {
			return nil, invalidInput(fmt.Errorf("botão %d: %w", index+1, err))
		}
		buttons = append(buttons, &waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
			Name:             proto.String(string(button.Type)),
			ButtonParamsJSON: proto.String(params),
		})
	}

	interactive := &waE2E.InteractiveMessage{
		Header: interactiveHeader(req.Header),
		Body:   &waE2E.InteractiveMessage_Body{Text: proto.String(req.Body)},
		Footer: interactiveFooter(req.Footer),
		InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
			NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
				Buttons:        buttons,
				MessageVersion: proto.Int32(1),
			},
		},
	}

	m.prepareSend(ctx, to, req.Body, 0, types.ChatPresenceMediaText)

	resp, err := m.sendWithBizNode(ctx, to, &waE2E.Message{InteractiveMessage: interactive})
	if err != nil {
		return nil, interactiveSendError("enviar botões", err)
	}
	return messageResponse(resp, to, "sent"), nil
}

// SendList envia uma ListMessage com menu expansível de seções e linhas
// (LIST_TYPE_SINGLE_SELECT). Aceita de 1 a 10 seções com 1 a 10 linhas cada;
// a seleção do destinatário chega no webhook com o RowID escolhido.
func (m *Manager) SendList(ctx context.Context, req contracts.SendListRequest) (*contracts.MessageResponse, error) {
	to, err := parseRecipient(req.To)
	if err != nil {
		return nil, invalidInput(err)
	}
	if len(req.Sections) < 1 || len(req.Sections) > 10 {
		return nil, invalidInput(fmt.Errorf("a lista aceita de 1 a 10 seções, recebido %d", len(req.Sections)))
	}

	sections := make([]*waE2E.ListMessage_Section, 0, len(req.Sections))
	for index, section := range req.Sections {
		if len(section.Rows) < 1 || len(section.Rows) > 10 {
			return nil, invalidInput(fmt.Errorf("seção %d aceita de 1 a 10 linhas, recebido %d", index+1, len(section.Rows)))
		}
		sections = append(sections, buildListSection(section))
	}

	list := &waE2E.ListMessage{
		Title:       proto.String(req.Title),
		Description: proto.String(req.Description),
		ButtonText:  proto.String(req.ButtonText),
		ListType:    waE2E.ListMessage_SINGLE_SELECT.Enum(),
		Sections:    sections,
	}
	if strings.TrimSpace(req.Footer) != "" {
		list.FooterText = proto.String(req.Footer)
	}

	m.prepareSend(ctx, to, req.Title+" "+req.Description, 0, types.ChatPresenceMediaText)

	resp, err := m.sendWithBizNode(ctx, to, &waE2E.Message{ListMessage: list})
	if err != nil {
		return nil, interactiveSendError("enviar lista", err)
	}
	return messageResponse(resp, to, "sent"), nil
}

// sendWithBizNode envia a mensagem injetando o nó <biz> com o interactive
// native_flow, exigido pelo protocolo Multi-Device para mensagens interativas.
func (m *Manager) sendWithBizNode(ctx context.Context, to types.JID, msg *waE2E.Message) (whatsmeow.SendResponse, error) {
	nodes := []waBinary.Node{{
		Tag: "biz",
		Content: []waBinary.Node{{
			Tag:     "interactive",
			Attrs:   waBinary.Attrs{"type": "native_flow", "v": "1"},
			Content: []waBinary.Node{{Tag: "native_flow", Attrs: waBinary.Attrs{"name": "mixed"}}},
		}},
	}}
	return m.sendWithRetry(ctx, to, msg, whatsmeow.SendRequestExtra{AdditionalNodes: &nodes})
}

// quickReplyParams são os ButtonParamsJSON do botão quick_reply.
type quickReplyParams struct {
	DisplayText string `json:"display_text"`
	ID          string `json:"id"`
}

// ctaURLParams são os ButtonParamsJSON do botão cta_url.
type ctaURLParams struct {
	DisplayText string `json:"display_text"`
	URL         string `json:"url"`
	MerchantURL string `json:"merchant_url"`
}

// ctaCopyParams são os ButtonParamsJSON do botão cta_copy.
type ctaCopyParams struct {
	DisplayText string `json:"display_text"`
	ID          string `json:"id"`
	CopyCode    string `json:"copy_code"`
}

// buildButtonParams valida o botão e serializa o ButtonParamsJSON por tipo.
func buildButtonParams(button contracts.ButtonItem) (string, error) {
	label := strings.TrimSpace(button.Label)
	if label == "" {
		return "", errors.New("label vazio")
	}
	id := strings.TrimSpace(button.ID)

	switch button.Type {
	case contracts.ButtonQuickReply:
		if id == "" {
			return "", errors.New("id é obrigatório para quick_reply")
		}
		return marshalButtonParams(quickReplyParams{DisplayText: label, ID: id})

	case contracts.ButtonCTAURL:
		value := strings.TrimSpace(button.Value)
		if value == "" {
			return "", errors.New("value (URL) é obrigatório para cta_url")
		}
		return marshalButtonParams(ctaURLParams{DisplayText: label, URL: value, MerchantURL: value})

	case contracts.ButtonCTACopy:
		value := strings.TrimSpace(button.Value)
		if id == "" {
			return "", errors.New("id é obrigatório para cta_copy")
		}
		if value == "" {
			return "", errors.New("value (código) é obrigatório para cta_copy")
		}
		return marshalButtonParams(ctaCopyParams{DisplayText: label, ID: id, CopyCode: value})

	default:
		return "", fmt.Errorf("tipo de botão desconhecido %q", button.Type)
	}
}

// marshalButtonParams serializa os parâmetros do botão para a string JSON
// esperada pelo campo ButtonParamsJSON do proto.
func marshalButtonParams(params any) (string, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("serializar ButtonParamsJSON: %w", err)
	}
	return string(data), nil
}

// buildListSection converte a seção do contrato na Section do proto,
// preservando a descrição da linha apenas quando informada.
func buildListSection(section contracts.ListSection) *waE2E.ListMessage_Section {
	rows := make([]*waE2E.ListMessage_Row, 0, len(section.Rows))
	for _, row := range section.Rows {
		entry := &waE2E.ListMessage_Row{
			RowID: proto.String(row.RowID),
			Title: proto.String(row.Title),
		}
		if strings.TrimSpace(row.Description) != "" {
			entry.Description = proto.String(row.Description)
		}
		rows = append(rows, entry)
	}
	return &waE2E.ListMessage_Section{
		Title: proto.String(section.Title),
		Rows:  rows,
	}
}

// interactiveHeader monta o header opcional da InteractiveMessage.
func interactiveHeader(title string) *waE2E.InteractiveMessage_Header {
	if strings.TrimSpace(title) == "" {
		return nil
	}
	return &waE2E.InteractiveMessage_Header{Title: proto.String(title)}
}

// interactiveFooter monta o footer opcional da InteractiveMessage.
func interactiveFooter(text string) *waE2E.InteractiveMessage_Footer {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return &waE2E.InteractiveMessage_Footer{Text: proto.String(text)}
}

// ErrInteractiveUnsupported indica que o servidor do WhatsApp recusou botões ou
// listas. Em contas pessoais o protocolo atual responde 473/479: mensagens
// NativeFlow/ListMessage são aceitas apenas em contas Business.
var ErrInteractiveUnsupported = errors.New("mensagens interativas não suportadas para esta conta")

// interactiveSendError traduz a recusa do servidor em erro acionável.
func interactiveSendError(operation string, err error) error {
	message := err.Error()
	if strings.Contains(message, "error 473") || strings.Contains(message, "error 479") {
		return fmt.Errorf("%w: %s: %v", ErrInteractiveUnsupported, operation, err)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
