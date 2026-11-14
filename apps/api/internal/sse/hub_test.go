package sse

import (
	"testing"
	"time"
)

func TestTicketsAreSingleUse(t *testing.T) {
	tickets := NewTickets(60 * time.Second)

	ticket, expires := tickets.Issue()
	if ticket == "" {
		t.Fatal("ticket vazio")
	}
	if time.Until(expires) <= 0 {
		t.Fatal("ticket já nasceu expirado")
	}

	if !tickets.Redeem(ticket) {
		t.Fatal("primeiro consumo deveria ser aceito")
	}
	if tickets.Redeem(ticket) {
		t.Fatal("ticket reutilizado deveria ser rejeitado")
	}
	if tickets.Redeem("") {
		t.Fatal("ticket vazio deveria ser rejeitado")
	}
	if tickets.Redeem("inexistente") {
		t.Fatal("ticket desconhecido deveria ser rejeitado")
	}
}

func TestTicketsExpire(t *testing.T) {
	tickets := NewTickets(time.Nanosecond)
	ticket, _ := tickets.Issue()
	time.Sleep(2 * time.Millisecond)
	if tickets.Redeem(ticket) {
		t.Fatal("ticket expirado deveria ser rejeitado")
	}
}

func TestCleanupRemovesExpired(t *testing.T) {
	tickets := NewTickets(time.Nanosecond)
	_, _ = tickets.Issue()
	time.Sleep(2 * time.Millisecond)
	tickets.Cleanup()

	tickets.mu.Lock()
	total := len(tickets.m)
	tickets.mu.Unlock()
	if total != 0 {
		t.Errorf("tickets restantes = %d; esperado 0", total)
	}
}

func TestHubBroadcastAndUnsubscribe(t *testing.T) {
	hub := NewHub(nil)

	first, cancelFirst := hub.Subscribe(4)
	defer cancelFirst()
	second, cancelSecond := hub.Subscribe(4)
	defer cancelSecond()

	if hub.Count() != 2 {
		t.Fatalf("Count = %d; esperado 2", hub.Count())
	}

	hub.Broadcast(map[string]string{"evento": "teste"})

	for _, ch := range []<-chan []byte{first, second} {
		select {
		case payload := <-ch:
			if string(payload) != "{\"evento\":\"teste\"}" {
				t.Errorf("payload = %s", payload)
			}
		case <-time.After(time.Second):
			t.Fatal("nenhum evento recebido")
		}
	}

	cancelFirst()
	if hub.Count() != 1 {
		t.Errorf("Count após cancelar = %d; esperado 1", hub.Count())
	}
	cancelFirst() // idempotente
}

func TestHubDoesNotBlockOnSlowClient(t *testing.T) {
	hub := NewHub(nil)
	_, cancel := hub.Subscribe(1)
	defer cancel()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			hub.Broadcast(map[string]int{"n": i})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Broadcast bloqueou com assinante lento")
	}
}
