package human

import (
	"context"
	"testing"
	"time"
)

func TestTypingDelayRespeitaLimites(t *testing.T) {
	profile := NewProfile(true, 6, 0, 0, 2*time.Second, 0.35)

	short := profile.TypingDelay("oi")
	if short < minTyping {
		t.Errorf("texto curto ficou instantâneo: %v", short)
	}
	if short > 2*time.Second {
		t.Errorf("texto curto passou do teto: %v", short)
	}

	long := profile.TypingDelay(string(make([]rune, 500)))
	if long > 2*time.Second {
		t.Errorf("texto longo deveria respeitar MaxTyping: %v", long)
	}
	if long < 1*time.Second {
		t.Errorf("texto longo deveria ser lento: %v", long)
	}
}

func TestTypingDelayEhIrregular(t *testing.T) {
	profile := NewProfile(true, 6, 0, 0, 8*time.Second, 0.5)

	seen := make(map[time.Duration]bool)
	for i := 0; i < 20; i++ {
		seen[profile.TypingDelay("mensagem de tamanho médio")] = true
	}
	if len(seen) < 2 {
		t.Errorf("esperava durações variadas (jitter), obtive %d valor(es)", len(seen))
	}
}

func TestProfileDesligadoZeraEsperas(t *testing.T) {
	profile := NewProfile(false, 6, time.Second, 2*time.Second, time.Second, 0.3)
	if got := profile.TypingDelay("qualquer texto"); got != 0 {
		t.Errorf("TypingDelay = %v, esperava 0", got)
	}
	if got := profile.Gap(); got != 0 {
		t.Errorf("Gap = %v, esperava 0", got)
	}
	if got := profile.Backoff(2); got != 0 {
		t.Errorf("Backoff = %v, esperava 0", got)
	}

	pacer := NewPacer(profile)
	start := time.Now()
	if err := pacer.Wait(context.Background(), "chat"); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("pacer desligado esperou %v", elapsed)
	}
}

func TestPacerEspacaPorConversa(t *testing.T) {
	profile := NewProfile(true, 6, 80*time.Millisecond, 80*time.Millisecond, time.Second, 0.01)
	pacer := NewPacer(profile)
	ctx := context.Background()

	if err := pacer.Wait(ctx, "chat-a"); err != nil {
		t.Fatalf("primeiro Wait: %v", err)
	}

	start := time.Now()
	if err := pacer.Wait(ctx, "chat-a"); err != nil {
		t.Fatalf("segundo Wait: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 60*time.Millisecond {
		t.Errorf("mesma conversa não respeitou o intervalo: %v", elapsed)
	}

	start = time.Now()
	if err := pacer.Wait(ctx, "chat-b"); err != nil {
		t.Fatalf("Wait em outra conversa: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 40*time.Millisecond {
		t.Errorf("conversa diferente não deveria esperar: %v", elapsed)
	}
}

func TestPacerRespeitaCancelamento(t *testing.T) {
	profile := NewProfile(true, 6, 2*time.Second, 2*time.Second, time.Second, 0.01)
	pacer := NewPacer(profile)
	_ = pacer.Wait(context.Background(), "chat")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := pacer.Wait(ctx, "chat"); err == nil {
		t.Error("esperava erro de contexto cancelado")
	}
}

func TestBackoffCresce(t *testing.T) {
	profile := NewProfile(true, 6, 0, 0, time.Second, 0.01)
	first := profile.Backoff(1)
	third := profile.Backoff(3)
	if first >= third {
		t.Errorf("backoff deveria crescer: %v >= %v", first, third)
	}
	if third > 8*time.Second {
		t.Errorf("backoff estourou o teto: %v", third)
	}
}
