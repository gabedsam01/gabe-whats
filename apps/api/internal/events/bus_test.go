package events

import (
	"testing"
	"time"

	"github.com/gabedsam01/gabe-whats/apps/api/internal/appstore"
	"github.com/gabedsam01/gabe-whats/shared/contracts"
)

func messageEvent(chat, sender string, fromMe bool) contracts.WebhookInboundEvent {
	return contracts.WebhookInboundEvent{
		EventID:   "evt",
		EventType: contracts.EventMessageReceived,
		ChatJID:   chat,
		SenderJID: sender,
		IsFromMe:  fromMe,
	}
}

func TestShouldDeliverIgnoreFromMe(t *testing.T) {
	settings := &appstore.WebhookSettings{IgnoreFromMe: true}

	if shouldDeliver(settings, messageEvent("a@s.whatsapp.net", "eu@s.whatsapp.net", true)) {
		t.Error("mensagem própria deveria ser filtrada")
	}
	if !shouldDeliver(settings, messageEvent("a@s.whatsapp.net", "a@s.whatsapp.net", false)) {
		t.Error("mensagem recebida deveria passar")
	}
}

func TestShouldDeliverAllowJIDs(t *testing.T) {
	settings := &appstore.WebhookSettings{
		IgnoreFromMe: true,
		AllowJIDs:    []string{"5511999999999@s.whatsapp.net"},
	}

	if !shouldDeliver(settings, messageEvent("5511999999999@s.whatsapp.net", "x@s.whatsapp.net", false)) {
		t.Error("chat na allowlist deveria passar")
	}
	if shouldDeliver(settings, messageEvent("5511888888888@s.whatsapp.net", "y@s.whatsapp.net", false)) {
		t.Error("chat fora da allowlist deveria ser filtrado")
	}
}

func TestShouldDeliverAlwaysAllowsNonMessageEvents(t *testing.T) {
	settings := &appstore.WebhookSettings{IgnoreFromMe: true, AllowJIDs: []string{"nada"}}
	event := contracts.WebhookInboundEvent{EventType: contracts.EventSessionConnected}

	if !shouldDeliver(settings, event) {
		t.Error("evento de sessão não deveria ser filtrado por allowlist")
	}
}

func TestRecentOrderingAndFilter(t *testing.T) {
	recent := NewRecent(3)

	for i, chat := range []string{"a", "a", "b"} {
		event := messageEvent(chat, "x", false)
		event.MessageID = string(rune('1' + i))
		event.Timestamp = time.Now().Add(time.Duration(i) * time.Second)
		recent.Add(event)
	}

	all := recent.List("", 10)
	if len(all) != 3 {
		t.Fatalf("len = %d; esperado 3", len(all))
	}
	if all[0].MessageID != "3" {
		t.Errorf("ordem invertida: primeiro = %q", all[0].MessageID)
	}

	onlyB := recent.List("b", 10)
	if len(onlyB) != 1 || onlyB[0].ChatJID != "b" {
		t.Errorf("filtro por chat falhou: %+v", onlyB)
	}

	if got := recent.List("", 2); len(got) != 2 {
		t.Errorf("limite ignorado: %d", len(got))
	}

	// Eventos que não são mensagens não entram no histórico.
	recent.Add(contracts.WebhookInboundEvent{EventType: contracts.EventSessionConnected})
	if got := recent.List("", 10); len(got) != 3 {
		t.Errorf("evento de sessão entrou no histórico: %d", len(got))
	}
}
