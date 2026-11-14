// Package tools registra as ferramentas MCP que encaminham operações à API REST.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gabedsam01/gabe-whats/apps/mcp/internal/apiclient"
	"github.com/gabedsam01/gabe-whats/shared/contracts"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// jsonResult serializa a resposta da API em JSON legível para o cliente MCP.
func jsonResult(value any) (*mcp.CallToolResult, any, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, nil, nil
}

type emptyInput struct{}

type pairPhoneInput struct {
	Phone string `json:"phone" jsonschema:"Número de telefone com DDI, apenas dígitos"`
}

type checkNumbersInput struct {
	Phones []string `json:"phones" jsonschema:"Números com DDI ou JIDs completos a verificar"`
}

type sendTextInput struct {
	To               string   `json:"to" jsonschema:"Número com DDI ou JID completo do destinatário"`
	Text             string   `json:"text" jsonschema:"Conteúdo da mensagem de texto"`
	ReplyToID        string   `json:"reply_to_id,omitempty" jsonschema:"ID opcional da mensagem respondida"`
	Mentions         []string `json:"mentions,omitempty" jsonschema:"JIDs ou números a mencionar"`
	SimulateTypingMs int      `json:"simulate_typing_ms,omitempty" jsonschema:"Duração opcional em milissegundos para simular digitação"`
}

type sendMediaInput struct {
	To        string `json:"to" jsonschema:"Número ou JID do destinatário"`
	MediaType string `json:"media_type" jsonschema:"Tipo de mídia: image, video ou document"`
	Source    string `json:"source" jsonschema:"URL, caminho local permitido ou conteúdo Base64"`
	MimeType  string `json:"mime_type,omitempty" jsonschema:"Tipo MIME da mídia, se conhecido"`
	Filename  string `json:"filename,omitempty" jsonschema:"Nome de arquivo opcional, recomendado para documentos"`
	Caption   string `json:"caption,omitempty" jsonschema:"Legenda opcional"`
	ReplyToID string `json:"reply_to_id,omitempty" jsonschema:"ID opcional da mensagem respondida"`
}

type sendVoiceInput struct {
	To        string `json:"to" jsonschema:"Número ou JID do destinatário"`
	Source    string `json:"source" jsonschema:"Áudio OGG/Opus via URL, caminho local ou Base64"`
	ReplyToID string `json:"reply_to_id,omitempty" jsonschema:"ID opcional da mensagem respondida"`
}

type buttonInput struct {
	Type  string `json:"type" jsonschema:"Tipo: quick_reply, cta_url ou cta_copy"`
	Label string `json:"label" jsonschema:"Texto exibido no botão"`
	ID    string `json:"id" jsonschema:"Identificador do botão"`
	Value string `json:"value,omitempty" jsonschema:"URL para cta_url ou código para cta_copy"`
}

type sendButtonsInput struct {
	To      string        `json:"to" jsonschema:"Número ou JID do destinatário"`
	Header  string        `json:"header,omitempty" jsonschema:"Cabeçalho opcional"`
	Body    string        `json:"body" jsonschema:"Corpo principal da mensagem"`
	Footer  string        `json:"footer,omitempty" jsonschema:"Rodapé opcional"`
	Buttons []buttonInput `json:"buttons" jsonschema:"Botões interativos (de 1 a 3)"`
}

type listRowInput struct {
	RowID       string `json:"row_id" jsonschema:"ID único da linha"`
	Title       string `json:"title" jsonschema:"Título da opção"`
	Description string `json:"description,omitempty" jsonschema:"Descrição curta opcional da opção"`
}

type listSectionInput struct {
	Title string         `json:"title" jsonschema:"Título da seção"`
	Rows  []listRowInput `json:"rows" jsonschema:"Opções da seção"`
}

type sendListInput struct {
	To          string             `json:"to" jsonschema:"Número ou JID do destinatário"`
	Title       string             `json:"title" jsonschema:"Título do menu"`
	Description string             `json:"description" jsonschema:"Texto explicativo da mensagem"`
	ButtonText  string             `json:"button_text" jsonschema:"Rótulo do botão que abre a lista"`
	Sections    []listSectionInput `json:"sections" jsonschema:"Seções com as opções do menu"`
}

type sendPollInput struct {
	To            string   `json:"to" jsonschema:"Número ou JID do destinatário"`
	Question      string   `json:"question" jsonschema:"Pergunta da enquete"`
	Options       []string `json:"options" jsonschema:"Opções da enquete (de 2 a 12)"`
	MaxSelections *int     `json:"max_selections,omitempty" jsonschema:"Limite de escolhas; omitido significa escolha única"`
}

type reactInput struct {
	ChatJID   string `json:"chat_jid" jsonschema:"JID do chat ou grupo"`
	MessageID string `json:"message_id" jsonschema:"ID da mensagem alvo"`
	Emoji     string `json:"emoji" jsonschema:"Emoji da reação; string vazia remove a reação"`
	SenderJID string `json:"sender_jid,omitempty" jsonschema:"JID do remetente da mensagem, obrigatório para alguns recibos em grupos"`
}

type editMessageInput struct {
	ChatJID   string `json:"chat_jid" jsonschema:"JID do chat"`
	MessageID string `json:"message_id" jsonschema:"ID da mensagem a editar"`
	NewText   string `json:"new_text" jsonschema:"Novo texto da mensagem"`
}

type revokeMessageInput struct {
	ChatJID   string `json:"chat_jid" jsonschema:"JID do chat"`
	MessageID string `json:"message_id" jsonschema:"ID da mensagem a revogar"`
}

type markReadInput struct {
	ChatJID    string   `json:"chat_jid" jsonschema:"JID do chat ou grupo"`
	MessageIDs []string `json:"message_ids" jsonschema:"IDs das mensagens a marcar como lidas"`
	SenderJID  string   `json:"sender_jid,omitempty" jsonschema:"JID do remetente, necessário para recibos de leitura em grupos"`
	Timestamp  string   `json:"timestamp,omitempty" jsonschema:"Instante opcional em formato RFC3339"`
}

type getGroupInput struct {
	GroupJID string `json:"group_jid" jsonschema:"JID do grupo terminado em @g.us"`
}

type createGroupInput struct {
	Name         string   `json:"name" jsonschema:"Nome do grupo (limite de 25 caracteres)"`
	Participants []string `json:"participants" jsonschema:"Números ou JIDs dos participantes iniciais"`
}

type updateParticipantsInput struct {
	GroupJID     string   `json:"group_jid" jsonschema:"JID do grupo"`
	Action       string   `json:"action" jsonschema:"Ação: add, remove, promote ou demote"`
	Participants []string `json:"participants" jsonschema:"Números ou JIDs dos participantes alvo"`
}

type updateGroupSettingsInput struct {
	GroupJID    string  `json:"group_jid" jsonschema:"JID do grupo"`
	Name        *string `json:"name,omitempty" jsonschema:"Novo nome opcional (limite de 25 caracteres)"`
	Description *string `json:"description,omitempty" jsonschema:"Nova descrição opcional"`
	Announce    *bool   `json:"announce,omitempty" jsonschema:"Se true, somente administradores podem enviar mensagens"`
	Locked      *bool   `json:"locked,omitempty" jsonschema:"Se true, somente administradores podem editar os dados do grupo"`
}

type inviteLinkInput struct {
	GroupJID string `json:"group_jid" jsonschema:"JID do grupo"`
	Reset    bool   `json:"reset,omitempty" jsonschema:"Se true, revoga o link atual e gera outro"`
}

type joinGroupInput struct {
	Invite string `json:"invite" jsonschema:"Código ou URL de convite do grupo"`
}

func requireNonEmpty(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s é obrigatório", field)
	}
	return nil
}

func requireStrings(field string, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s deve conter ao menos um item", field)
	}
	for index, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s[%d] não pode ser vazio", field, index)
		}
	}
	return nil
}

// Register registra as ferramentas MCP e as conecta ao cliente REST.
func Register(server *mcp.Server, client *apiclient.Client) {
	mcp.AddTool[emptyInput, any](server, &mcp.Tool{
		Name:        "whatsapp_session_status",
		Description: "Consulta se a sessão do WhatsApp está conectada e autenticada, incluindo os dados disponíveis do número pareado.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
		resp, err := client.Status(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[emptyInput, any](server, &mcp.Tool{
		Name:        "whatsapp_get_qr",
		Description: "Obtém o estado atual do pareamento por QR Code. Quando disponível, retorna o código e a imagem PNG em Base64; o QR pode expirar.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
		resp, err := client.QRCode(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[pairPhoneInput, any](server, &mcp.Tool{
		Name:        "whatsapp_pair_phone",
		Description: "Gera um código de pareamento para vincular este dispositivo ao WhatsApp usando o número informado com DDI.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in pairPhoneInput) (*mcp.CallToolResult, any, error) {
		if err := requireNonEmpty("phone", in.Phone); err != nil {
			return nil, nil, err
		}
		resp, err := client.PairCode(ctx, contracts.PairCodeRequest{Phone: in.Phone})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[checkNumbersInput, any](server, &mcp.Tool{
		Name:        "whatsapp_check_numbers",
		Description: "Verifica quais números informados possuem uma conta registrada no WhatsApp. Informe números com DDI ou JIDs completos.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in checkNumbersInput) (*mcp.CallToolResult, any, error) {
		if err := requireStrings("phones", in.Phones); err != nil {
			return nil, nil, err
		}
		resp, err := client.CheckNumbers(ctx, contracts.CheckNumbersRequest{Phones: in.Phones})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[sendTextInput, any](server, &mcp.Tool{
		Name:        "whatsapp_send_text",
		Description: "Envia uma mensagem de texto para um número com DDI ou JID. Opcionalmente responde a uma mensagem, menciona contatos ou simula digitação antes do envio.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in sendTextInput) (*mcp.CallToolResult, any, error) {
		if err := requireNonEmpty("to", in.To); err != nil {
			return nil, nil, err
		}
		if err := requireNonEmpty("text", in.Text); err != nil {
			return nil, nil, err
		}
		resp, err := client.SendText(ctx, contracts.SendTextRequest{
			To: in.To, Text: in.Text, ReplyToID: in.ReplyToID,
			Mentions: in.Mentions, SimulateTypingMs: in.SimulateTypingMs,
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[sendMediaInput, any](server, &mcp.Tool{
		Name:        "whatsapp_send_media",
		Description: "Envia imagem, vídeo ou documento. A origem pode ser URL, caminho local permitido ou Base64; tipo MIME, nome, legenda e resposta citada são opcionais.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in sendMediaInput) (*mcp.CallToolResult, any, error) {
		if err := requireNonEmpty("to", in.To); err != nil {
			return nil, nil, err
		}
		if err := requireNonEmpty("source", in.Source); err != nil {
			return nil, nil, err
		}
		switch in.MediaType {
		case "image", "video", "document":
		default:
			return nil, nil, fmt.Errorf("media_type deve ser image, video ou document")
		}
		resp, err := client.SendMedia(ctx, contracts.SendMediaRequest{
			To: in.To, MediaType: in.MediaType, Source: in.Source, MimeType: in.MimeType,
			Filename: in.Filename, Caption: in.Caption, ReplyToID: in.ReplyToID,
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[sendVoiceInput, any](server, &mcp.Tool{
		Name:        "whatsapp_send_voice_ptt",
		Description: "Envia áudio OGG/Opus como nota de voz PTT. A origem pode ser URL, caminho local permitido ou conteúdo Base64.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in sendVoiceInput) (*mcp.CallToolResult, any, error) {
		if err := requireNonEmpty("to", in.To); err != nil {
			return nil, nil, err
		}
		if err := requireNonEmpty("source", in.Source); err != nil {
			return nil, nil, err
		}
		resp, err := client.SendVoice(ctx, contracts.SendVoiceRequest{To: in.To, Source: in.Source, ReplyToID: in.ReplyToID})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[sendButtonsInput, any](server, &mcp.Tool{
		Name:        "whatsapp_send_buttons",
		Description: "Envia uma mensagem com botões interativos quick_reply, cta_url ou cta_copy (de 1 a 3). O WhatsApp pode recusar botões em conta pessoal com interactive_unsupported; nesse caso, use whatsapp_send_poll como alternativa nativa.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in sendButtonsInput) (*mcp.CallToolResult, any, error) {
		if err := requireNonEmpty("to", in.To); err != nil {
			return nil, nil, err
		}
		if err := requireNonEmpty("body", in.Body); err != nil {
			return nil, nil, err
		}
		if len(in.Buttons) == 0 || len(in.Buttons) > 3 {
			return nil, nil, fmt.Errorf("buttons deve conter de 1 a 3 itens")
		}
		buttons := make([]contracts.ButtonItem, len(in.Buttons))
		for index, button := range in.Buttons {
			if err := requireNonEmpty(fmt.Sprintf("buttons[%d].type", index), button.Type); err != nil {
				return nil, nil, err
			}
			if err := requireNonEmpty(fmt.Sprintf("buttons[%d].label", index), button.Label); err != nil {
				return nil, nil, err
			}
			if err := requireNonEmpty(fmt.Sprintf("buttons[%d].id", index), button.ID); err != nil {
				return nil, nil, err
			}
			switch button.Type {
			case string(contracts.ButtonQuickReply), string(contracts.ButtonCTAURL), string(contracts.ButtonCTACopy):
			default:
				return nil, nil, fmt.Errorf("buttons[%d].type inválido: use quick_reply, cta_url ou cta_copy", index)
			}
			buttons[index] = contracts.ButtonItem{Type: contracts.ButtonType(button.Type), Label: button.Label, ID: button.ID, Value: button.Value}
		}
		resp, err := client.SendButtons(ctx, contracts.SendButtonsRequest{
			To: in.To, Header: in.Header, Body: in.Body, Footer: in.Footer, Buttons: buttons,
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[sendListInput, any](server, &mcp.Tool{
		Name:        "whatsapp_send_list",
		Description: "Envia um menu interativo com seções e linhas selecionáveis. O WhatsApp pode recusar listas em conta pessoal com interactive_unsupported; nesse caso, use whatsapp_send_poll como alternativa nativa.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in sendListInput) (*mcp.CallToolResult, any, error) {
		for field, value := range map[string]string{"to": in.To, "title": in.Title, "description": in.Description, "button_text": in.ButtonText} {
			if err := requireNonEmpty(field, value); err != nil {
				return nil, nil, err
			}
		}
		if len(in.Sections) == 0 || len(in.Sections) > 10 {
			return nil, nil, fmt.Errorf("sections deve conter de 1 a 10 itens")
		}
		sections := make([]contracts.ListSection, len(in.Sections))
		for sectionIndex, section := range in.Sections {
			if err := requireNonEmpty(fmt.Sprintf("sections[%d].title", sectionIndex), section.Title); err != nil {
				return nil, nil, err
			}
			if len(section.Rows) == 0 || len(section.Rows) > 10 {
				return nil, nil, fmt.Errorf("sections[%d].rows deve conter de 1 a 10 itens", sectionIndex)
			}
			rows := make([]contracts.ListRow, len(section.Rows))
			for rowIndex, row := range section.Rows {
				if err := requireNonEmpty(fmt.Sprintf("sections[%d].rows[%d].row_id", sectionIndex, rowIndex), row.RowID); err != nil {
					return nil, nil, err
				}
				if err := requireNonEmpty(fmt.Sprintf("sections[%d].rows[%d].title", sectionIndex, rowIndex), row.Title); err != nil {
					return nil, nil, err
				}
				rows[rowIndex] = contracts.ListRow{RowID: row.RowID, Title: row.Title, Description: row.Description}
			}
			sections[sectionIndex] = contracts.ListSection{Title: section.Title, Rows: rows}
		}
		resp, err := client.SendList(ctx, contracts.SendListRequest{
			To: in.To, Title: in.Title, Description: in.Description,
			ButtonText: in.ButtonText, Sections: sections,
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[sendPollInput, any](server, &mcp.Tool{
		Name:        "whatsapp_send_poll",
		Description: "Envia uma enquete nativa do WhatsApp, alternativa compatível com conta pessoal quando botões ou listas interativos são recusados. Informe de 2 a 12 opções; max_selections é opcional.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in sendPollInput) (*mcp.CallToolResult, any, error) {
		if err := requireNonEmpty("to", in.To); err != nil {
			return nil, nil, err
		}
		if err := requireNonEmpty("question", in.Question); err != nil {
			return nil, nil, err
		}
		if len(in.Options) < 2 || len(in.Options) > 12 {
			return nil, nil, fmt.Errorf("options deve conter de 2 a 12 itens")
		}
		if err := requireStrings("options", in.Options); err != nil {
			return nil, nil, err
		}
		resp, err := client.SendPoll(ctx, contracts.SendPollRequest{
			To: in.To, Question: in.Question, Options: in.Options, MaxSelections: in.MaxSelections,
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[reactInput, any](server, &mcp.Tool{
		Name:        "whatsapp_react_message",
		Description: "Adiciona ou remove uma reação em uma mensagem. Informe uma string vazia em emoji para remover; em grupos, sender_jid pode ser necessário.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in reactInput) (*mcp.CallToolResult, any, error) {
		if err := requireNonEmpty("chat_jid", in.ChatJID); err != nil {
			return nil, nil, err
		}
		if err := requireNonEmpty("message_id", in.MessageID); err != nil {
			return nil, nil, err
		}
		resp, err := client.React(ctx, contracts.ReactionRequest{
			ChatJID: in.ChatJID, MessageID: in.MessageID, Emoji: in.Emoji, SenderJID: in.SenderJID,
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[editMessageInput, any](server, &mcp.Tool{
		Name:        "whatsapp_edit_message",
		Description: "Edita o texto de uma mensagem enviada anteriormente. Informe o JID do chat, o ID da mensagem e o novo texto.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in editMessageInput) (*mcp.CallToolResult, any, error) {
		for field, value := range map[string]string{"chat_jid": in.ChatJID, "message_id": in.MessageID, "new_text": in.NewText} {
			if err := requireNonEmpty(field, value); err != nil {
				return nil, nil, err
			}
		}
		resp, err := client.EditMessage(ctx, in.MessageID, contracts.EditMessageRequest{ChatJID: in.ChatJID, NewText: in.NewText})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[revokeMessageInput, any](server, &mcp.Tool{
		Name:        "whatsapp_revoke_message",
		Description: "Revoga uma mensagem enviada, solicitando que ela seja apagada para todos no chat.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in revokeMessageInput) (*mcp.CallToolResult, any, error) {
		if err := requireNonEmpty("chat_jid", in.ChatJID); err != nil {
			return nil, nil, err
		}
		if err := requireNonEmpty("message_id", in.MessageID); err != nil {
			return nil, nil, err
		}
		resp, err := client.RevokeMessage(ctx, in.MessageID, contracts.RevokeMessageRequest{ChatJID: in.ChatJID})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[markReadInput, any](server, &mcp.Tool{
		Name:        "whatsapp_mark_read",
		Description: "Marca uma ou mais mensagens como lidas. O timestamp opcional deve estar no formato RFC3339; em grupos, informe sender_jid quando exigido pelo recibo.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in markReadInput) (*mcp.CallToolResult, any, error) {
		if err := requireNonEmpty("chat_jid", in.ChatJID); err != nil {
			return nil, nil, err
		}
		if err := requireStrings("message_ids", in.MessageIDs); err != nil {
			return nil, nil, err
		}
		var timestamp *time.Time
		if in.Timestamp != "" {
			parsed, err := time.Parse(time.RFC3339, in.Timestamp)
			if err != nil {
				return nil, nil, fmt.Errorf("timestamp deve estar no formato RFC3339: %w", err)
			}
			timestamp = &parsed
		}
		resp, err := client.MarkRead(ctx, contracts.MarkReadRequest{
			ChatJID: in.ChatJID, MessageIDs: in.MessageIDs, SenderJID: in.SenderJID, Timestamp: timestamp,
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[emptyInput, any](server, &mcp.Tool{
		Name:        "whatsapp_list_groups",
		Description: "Lista os grupos de WhatsApp dos quais a sessão participa, incluindo metadados e participantes disponíveis.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
		resp, err := client.ListGroups(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[getGroupInput, any](server, &mcp.Tool{
		Name:        "whatsapp_get_group",
		Description: "Obtém os metadados e participantes de um grupo identificado pelo JID.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getGroupInput) (*mcp.CallToolResult, any, error) {
		if err := requireNonEmpty("group_jid", in.GroupJID); err != nil {
			return nil, nil, err
		}
		resp, err := client.GetGroup(ctx, in.GroupJID)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[createGroupInput, any](server, &mcp.Tool{
		Name:        "whatsapp_create_group",
		Description: "Cria um grupo de WhatsApp com nome de até 25 caracteres e os participantes iniciais informados.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createGroupInput) (*mcp.CallToolResult, any, error) {
		if err := requireNonEmpty("name", in.Name); err != nil {
			return nil, nil, err
		}
		if len([]rune(in.Name)) > 25 {
			return nil, nil, fmt.Errorf("name não pode exceder 25 caracteres")
		}
		if err := requireStrings("participants", in.Participants); err != nil {
			return nil, nil, err
		}
		resp, err := client.CreateGroup(ctx, contracts.CreateGroupRequest{Name: in.Name, Participants: in.Participants})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[updateParticipantsInput, any](server, &mcp.Tool{
		Name:        "whatsapp_update_group_participants",
		Description: "Adiciona, remove, promove ou rebaixa participantes de um grupo. A resposta contém o resultado individual para cada participante; um participante pode falhar sem que os demais falhem.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateParticipantsInput) (*mcp.CallToolResult, any, error) {
		if err := requireNonEmpty("group_jid", in.GroupJID); err != nil {
			return nil, nil, err
		}
		if err := requireStrings("participants", in.Participants); err != nil {
			return nil, nil, err
		}
		switch in.Action {
		case string(contracts.GroupActionAdd), string(contracts.GroupActionRemove), string(contracts.GroupActionPromote), string(contracts.GroupActionDemote):
		default:
			return nil, nil, fmt.Errorf("action deve ser add, remove, promote ou demote")
		}
		resp, err := client.UpdateGroupParticipants(ctx, in.GroupJID, contracts.UpdateGroupParticipantsRequest{
			Action: contracts.GroupParticipantAction(in.Action), Participants: in.Participants,
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[updateGroupSettingsInput, any](server, &mcp.Tool{
		Name:        "whatsapp_update_group_settings",
		Description: "Atualiza nome, descrição, permissão de envio (announce) ou bloqueio de edição (locked) de um grupo. Informe pelo menos uma configuração.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateGroupSettingsInput) (*mcp.CallToolResult, any, error) {
		if err := requireNonEmpty("group_jid", in.GroupJID); err != nil {
			return nil, nil, err
		}
		if in.Name == nil && in.Description == nil && in.Announce == nil && in.Locked == nil {
			return nil, nil, fmt.Errorf("informe ao menos uma configuração para atualizar")
		}
		if in.Name != nil && len([]rune(*in.Name)) > 25 {
			return nil, nil, fmt.Errorf("name não pode exceder 25 caracteres")
		}
		resp, err := client.UpdateGroupSettings(ctx, in.GroupJID, contracts.UpdateGroupSettingsRequest{
			Name: in.Name, Description: in.Description, Announce: in.Announce, Locked: in.Locked,
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[inviteLinkInput, any](server, &mcp.Tool{
		Name:        "whatsapp_get_group_invite_link",
		Description: "Obtém o link de convite atual de um grupo. Com reset=true, revoga o link anterior e gera um novo.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in inviteLinkInput) (*mcp.CallToolResult, any, error) {
		if err := requireNonEmpty("group_jid", in.GroupJID); err != nil {
			return nil, nil, err
		}
		resp, err := client.GroupInviteLink(ctx, in.GroupJID, in.Reset)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[joinGroupInput, any](server, &mcp.Tool{
		Name:        "whatsapp_join_group",
		Description: "Entra em um grupo usando o código ou a URL de convite do WhatsApp.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in joinGroupInput) (*mcp.CallToolResult, any, error) {
		if err := requireNonEmpty("invite", in.Invite); err != nil {
			return nil, nil, err
		}
		resp, err := client.JoinGroup(ctx, in.Invite)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})

	mcp.AddTool[getGroupInput, any](server, &mcp.Tool{
		Name:        "whatsapp_leave_group",
		Description: "Sai do grupo identificado pelo JID. Esta ação remove a sessão do grupo.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getGroupInput) (*mcp.CallToolResult, any, error) {
		if err := requireNonEmpty("group_jid", in.GroupJID); err != nil {
			return nil, nil, err
		}
		resp, err := client.LeaveGroup(ctx, in.GroupJID)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(resp)
	})
}
