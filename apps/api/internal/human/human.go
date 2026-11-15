// Package human aplica um ritmo irregular ao envio de mensagens.
//
// O princípio vem do combate a anti-bot: "bot é regular, pessoa mexendo é
// irregular". Nada aqui altera o protocolo — apenas o tempo entre ações, que é
// justamente o que um servidor consegue medir com facilidade.
package human

import (
	"context"
	"math/rand"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	// defaultCharsPerSecond aproxima uma digitação humana confortável.
	defaultCharsPerSecond = 6.0
	// defaultJitter é a variação relativa aplicada a qualquer espera.
	defaultJitter = 0.35
	// defaultMaxTyping limita o tempo "digitando" mesmo em textos longos.
	defaultMaxTyping = 8 * time.Second
	// minTyping evita indicadores instantâneos, típicos de automação.
	minTyping = 600 * time.Millisecond
)

// Profile descreve o comportamento temporal configurado.
type Profile struct {
	Enabled        bool
	CharsPerSecond float64
	MinGap         time.Duration
	MaxGap         time.Duration
	MaxTyping      time.Duration
	Jitter         float64

	mu  sync.Mutex
	rnd *rand.Rand
}

// NewProfile cria o perfil com padrões seguros para campos zerados.
func NewProfile(enabled bool, charsPerSecond float64, minGap, maxGap, maxTyping time.Duration, jitter float64) *Profile {
	if charsPerSecond <= 0 {
		charsPerSecond = defaultCharsPerSecond
	}
	if maxTyping <= 0 {
		maxTyping = defaultMaxTyping
	}
	if jitter <= 0 || jitter >= 1 {
		jitter = defaultJitter
	}
	if maxGap < minGap {
		maxGap = minGap
	}
	return &Profile{
		Enabled:        enabled,
		CharsPerSecond: charsPerSecond,
		MinGap:         minGap,
		MaxGap:         maxGap,
		MaxTyping:      maxTyping,
		Jitter:         jitter,
		rnd:            rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// float devolve um número pseudoaleatório protegido por mutex.
func (p *Profile) float() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.rnd.Float64()
}

// withJitter aplica variação relativa em torno de uma duração base.
func (p *Profile) withJitter(base time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	factor := 1 + (p.float()*2-1)*p.Jitter
	return time.Duration(float64(base) * factor)
}

// TypingDelay estima quanto tempo uma pessoa levaria para escrever o texto.
// Texto curto não fica instantâneo e texto longo respeita o teto de MaxTyping.
func (p *Profile) TypingDelay(text string) time.Duration {
	if p == nil || !p.Enabled {
		return 0
	}

	chars := utf8.RuneCountInString(strings.TrimSpace(text))
	if chars == 0 {
		chars = 1
	}

	base := time.Duration(float64(chars) / p.CharsPerSecond * float64(time.Second))
	delay := p.withJitter(base)

	if delay < minTyping {
		delay = minTyping
	}
	if delay > p.MaxTyping {
		delay = p.MaxTyping
	}
	return delay
}

// Gap sorteia o intervalo mínimo antes de falar de novo na mesma conversa.
func (p *Profile) Gap() time.Duration {
	if p == nil || !p.Enabled {
		return 0
	}
	span := p.MaxGap - p.MinGap
	if span <= 0 {
		return p.MinGap
	}
	return p.MinGap + time.Duration(p.float()*float64(span))
}

// Backoff devolve a espera da tentativa informada (1 = primeira repetição),
// com crescimento exponencial e jitter.
func (p *Profile) Backoff(attempt int) time.Duration {
	if p == nil || !p.Enabled {
		return 0
	}
	if attempt < 1 {
		attempt = 1
	}
	base := 500 * time.Millisecond
	for i := 1; i < attempt; i++ {
		base *= 2
	}
	if base > 8*time.Second {
		base = 8 * time.Second
	}
	return p.withJitter(base)
}

// Pacer serializa e espaça envios por conversa.
type Pacer struct {
	profile *Profile
	mu      sync.Mutex
	next    map[string]time.Time
}

// NewPacer cria o controlador de ritmo.
func NewPacer(profile *Profile) *Pacer {
	return &Pacer{profile: profile, next: make(map[string]time.Time)}
}

// Wait bloqueia até o próximo horário liberado para a conversa e reserva a vez.
func (p *Pacer) Wait(ctx context.Context, chat string) error {
	if p == nil || p.profile == nil || !p.profile.Enabled {
		return nil
	}

	p.mu.Lock()
	now := time.Now()
	readyAt := p.next[chat]
	if readyAt.Before(now) {
		readyAt = now
	}
	wait := readyAt.Sub(now)
	p.next[chat] = readyAt.Add(p.profile.Gap())
	p.mu.Unlock()

	if wait <= 0 {
		return nil
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
