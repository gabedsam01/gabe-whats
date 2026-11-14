package whatsapp

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"

	"github.com/gabedsam01/gabe-whats/apps/api/internal/appstore"
	"github.com/gabedsam01/gabe-whats/shared/contracts"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

// SendPoll envia uma enquete nativa e guarda a criação no cache para que os
// votos recebidos possam ser convertidos de hash para texto.
func (m *Manager) SendPoll(ctx context.Context, req contracts.SendPollRequest) (*contracts.MessageResponse, error) {
	to, err := parseRecipient(req.To)
	if err != nil {
		return nil, invalidInput(err)
	}
	if len(req.Options) < 2 || len(req.Options) > 12 {
		return nil, invalidInput(fmt.Errorf("enquete deve ter de 2 a 12 opções, recebido %d", len(req.Options)))
	}

	selections := 1
	if req.MaxSelections != nil {
		selections = *req.MaxSelections
	}
	if selections < 0 || selections > len(req.Options) {
		return nil, invalidInput(fmt.Errorf("max_selections inválido: %d", selections))
	}

	message := m.client.BuildPollCreation(req.Question, req.Options, selections)
	resp, err := m.client.SendMessage(ctx, to, message)
	if err != nil {
		return nil, fmt.Errorf("enviar enquete: %w", err)
	}

	if m.app != nil {
		_ = m.app.SavePoll(ctx, appstore.PollRecord{
			MessageID:             resp.ID,
			ChatJID:               to.String(),
			Question:              req.Question,
			Options:               req.Options,
			SelectableOptionCount: selections,
		})
	}

	return messageResponse(resp, to, "sent"), nil
}

// cachePollCreation guarda a enquete recebida (âncora do cache de hashes).
func (m *Manager) cachePollCreation(ctx context.Context, poll *contracts.WebhookPollCreation, chatJID string) {
	if m.app == nil || poll == nil {
		return
	}
	_ = m.app.SavePoll(ctx, appstore.PollRecord{
		MessageID:             poll.PollMessageID,
		ChatJID:               chatJID,
		Question:              poll.Question,
		Options:               poll.Options,
		SelectableOptionCount: poll.SelectableOptionCount,
		PollSenderJID:         poll.PollSenderJID,
	})
}

// buildPollVote decifra o voto e resolve os hashes SHA-256 em textos usando o
// cache persistido. Sem cache, devolve ao menos os hashes (sempre presentes).
func (m *Manager) buildPollVote(ctx context.Context, evt *events.Message) *contracts.WebhookPollVote {
	update := evt.Message.GetPollUpdateMessage()
	if update == nil {
		return nil
	}

	vote := &contracts.WebhookPollVote{
		PollMessageID:  update.GetPollCreationMessageKey().GetID(),
		VoterJID:       evt.Info.Sender.String(),
		SelectedHashes: []string{},
	}

	decrypted, err := m.client.DecryptPollVote(ctx, evt)
	if err != nil {
		m.client.Log.Warnf("falha ao decifrar voto de enquete: %v", err)
		return vote
	}

	selected := decrypted.GetSelectedOptions()
	for _, hash := range selected {
		vote.SelectedHashes = append(vote.SelectedHashes, hex.EncodeToString(hash))
	}

	if m.app == nil {
		return vote
	}

	record, err := m.app.GetPoll(ctx, vote.PollMessageID)
	if err != nil {
		m.client.Log.Debugf("enquete não encontrada no cache: %s", vote.PollMessageID)
		return vote
	}

	vote.Question = record.Question
	vote.PollSenderJID = record.PollSenderJID

	hashes := whatsmeow.HashPollOptions(record.Options)
	for _, want := range selected {
		for index, candidate := range hashes {
			if int(index) < len(record.Options) && bytes.Equal(candidate, want) {
				vote.SelectedOptions = append(vote.SelectedOptions, record.Options[index])
				break
			}
		}
	}

	return vote
}
