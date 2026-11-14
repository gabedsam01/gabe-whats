// Package webhook entrega eventos normalizados a agentes por HTTP POST, com
// assinatura HMAC-SHA256 e política de reentrega.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gabedsam01/gabe-whats/shared/contracts"
)

// Policy descreve quando uma entrega é considerada falha e como reenviar.
//
// O contrato atual (definido pelo dono do projeto) é:
//   - a primeira requisição sai imediatamente;
//   - se o destino não responder com sucesso em Timeout, a tentativa é
//     considerada falha e a próxima é agendada;
//   - os intervalos entre tentativas são Delays, nesta ordem.
//
// Com os valores padrão são 6 requisições no total (1 + 5 reentregas), com
// esperas de 5s, 15s, 25s, 45s e 60s.
type Policy struct {
	// Attempts é o número total de requisições, incluindo a inicial.
	Attempts int
	// Timeout é o tempo máximo para considerar a tentativa falha.
	Timeout time.Duration
	// Delays é a espera antes de cada reentrega (len(Delays) == Attempts-1).
	Delays []time.Duration
}

// DefaultPolicy é a política acordada para o gabe-whats.
var DefaultPolicy = Policy{
	Attempts: 6,
	Timeout:  2 * time.Second,
	Delays: []time.Duration{
		5 * time.Second,
		15 * time.Second,
		25 * time.Second,
		45 * time.Second,
		60 * time.Second,
	},
}

// Validate garante a coerência da política.
func (p Policy) Validate() error {
	if p.Attempts < 1 {
		return errors.New("webhook: Attempts deve ser >= 1")
	}
	if p.Timeout <= 0 {
		return errors.New("webhook: Timeout deve ser positivo")
	}
	if len(p.Delays) != p.Attempts-1 {
		return fmt.Errorf("webhook: esperado %d intervalos, obtido %d", p.Attempts-1, len(p.Delays))
	}
	return nil
}

// DeliveryResult resume o desfecho de uma entrega.
type DeliveryResult struct {
	EventID    string
	Attempts   int
	StatusCode int
	Err        error
}

// Succeeded informa se a entrega obteve resposta de sucesso.
func (r DeliveryResult) Succeeded() bool { return r.Err == nil }

// Dispatcher entrega eventos usando um cliente HTTP e uma política.
type Dispatcher struct {
	policy Policy
	client *http.Client
	sleep  func(context.Context, time.Duration) error
	log    *slog.Logger
}

// New cria um Dispatcher com a política informada. Política inválida é erro.
func New(policy Policy, client *http.Client, logger *slog.Logger) (*Dispatcher, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Dispatcher{
		policy: policy,
		client: client,
		sleep:  sleepContext,
		log:    logger,
	}, nil
}

// Deliver envia o evento e, em caso de falha, reentrega conforme a política.
// Retorna o resultado final; eventuais falhas parciais ficam no log.
func (d *Dispatcher) Deliver(ctx context.Context, cfg contracts.WebhookConfig, event contracts.WebhookInboundEvent) DeliveryResult {
	result := DeliveryResult{EventID: event.EventID}

	if cfg.URL == "" {
		result.Err = errors.New("webhook: URL não configurada")
		return result
	}

	body, err := json.Marshal(event)
	if err != nil {
		result.Err = fmt.Errorf("webhook: serializar evento: %w", err)
		return result
	}

	signature := sign(cfg.Secret, body)

	for attempt := 0; attempt < d.policy.Attempts; attempt++ {
		if attempt > 0 {
			delay := d.policy.Delays[attempt-1]
			if err := d.sleep(ctx, delay); err != nil {
				result.Err = err
				result.Attempts = attempt
				return result
			}
		}

		status, err := d.post(ctx, cfg.URL, body, signature, event)
		result.Attempts = attempt + 1
		result.StatusCode = status

		if err == nil {
			result.Err = nil
			d.log.Debug("webhook entregue", "event_id", event.EventID, "tentativa", attempt+1, "status", status)
			return result
		}

		result.Err = err
		d.log.Warn("webhook falhou", "event_id", event.EventID, "tentativa", attempt+1, "status", status, "erro", err)
	}

	return result
}

// post executa uma tentativa, respeitando o timeout da política.
func (d *Dispatcher) post(ctx context.Context, url string, body []byte, signature string, event contracts.WebhookInboundEvent) (int, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, d.policy.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("montar requisição: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "gabe-whats/webhook")
	req.Header.Set("X-GabeWhats-Signature", signature)
	req.Header.Set("X-GabeWhats-Event", string(event.EventType))
	req.Header.Set("X-GabeWhats-Delivery", event.EventID)

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp.StatusCode, fmt.Errorf("status inesperado %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

// sign devolve o valor do header de assinatura (sha256=<hex>).
func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// sleepContext espera d ou o cancelamento do contexto.
func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
