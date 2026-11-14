// Package whatsapp encapsula o cliente go.mau.fi/whatsmeow atrás de uma API
// pequena e estável consumida pelos handlers HTTP. Nenhum tipo do whatsmeow
// vaza para fora deste pacote: a fronteira é o pacote shared/contracts.
package whatsapp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gabedsam01/gabe-whats/apps/api/internal/appstore"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/config"
	"github.com/gabedsam01/gabe-whats/shared/contracts"
	qrcode "github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

const (
	// pairCodeTTL é a janela de validade do código de pareamento informada pelo
	// upstream (a conexão de login é encerrada após ~160s).
	pairCodeTTL = 160 * time.Second
	// pairingTimeout limita a espera pelo primeiro QR antes de pedir o código.
	pairingTimeout = 15 * time.Second
	// pairPollInterval é o intervalo de checagem do estado de pareamento.
	pairPollInterval = 50 * time.Millisecond
)

var (
	// ErrAlreadyPaired indica que o dispositivo já está pareado e logado.
	ErrAlreadyPaired = errors.New("dispositivo já pareado")
	// ErrPairingInactive indica que nenhum processo de pareamento está ativo.
	ErrPairingInactive = errors.New("pareamento não iniciado")
	// ErrPairingTimeout indica que o QR não chegou a tempo.
	ErrPairingTimeout = errors.New("tempo esgotado aguardando o QR Code")
)

// Publisher recebe eventos normalizados (webhook/SSE/histórico).
type Publisher interface {
	Publish(ctx context.Context, event contracts.WebhookInboundEvent)
}

// MediaSink armazena mídias baixadas e devolve a referência com URL temporária.
type MediaSink interface {
	Save(ctx context.Context, data []byte, mimeType, filename string) (*contracts.MediaRef, error)
}

// Manager é o dono exclusivo da sessão do WhatsApp neste processo.
type Manager struct {
	client    *whatsmeow.Client
	container *sqlstore.Container
	startedAt time.Time

	cfg       *config.Config
	publisher Publisher
	media     MediaSink
	app       *appstore.Store

	pairMu sync.Mutex
	pair   *pairingState
}

type pairingState struct {
	mu      sync.Mutex
	latest  contracts.QRCodeResponse
	hasCode bool
	done    chan struct{}
	once    sync.Once
	err     error
}

func (p *pairingState) set(resp contracts.QRCodeResponse) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.latest = resp
	if resp.Event == "code" {
		p.hasCode = true
	}
}

func (p *pairingState) snapshot() (contracts.QRCodeResponse, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.latest, p.hasCode
}

func (p *pairingState) finish(err error) {
	p.once.Do(func() {
		p.mu.Lock()
		p.err = err
		p.mu.Unlock()
		close(p.done)
	})
}

// New abre o session.db, recupera o dispositivo e prepara o cliente. Quando já
// existe sessão, a conexão é iniciada em background para não bloquear o boot.
func New(ctx context.Context, cfg *config.Config, publisher Publisher, mediaSink MediaSink, app *appstore.Store) (*Manager, error) {
	logger := waLog.Stdout("WhatsApp", strings.ToLower(cfg.LogLevel), true)

	container, err := sqlstore.New(ctx, "sqlite3", cfg.SQLiteDSN, logger)
	if err != nil {
		return nil, fmt.Errorf("abrir session.db: %w", err)
	}

	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		_ = container.Close()
		return nil, fmt.Errorf("recuperar dispositivo: %w", err)
	}

	client := whatsmeow.NewClient(device, logger)
	m := &Manager{
		client:    client,
		container: container,
		startedAt: time.Now(),
		cfg:       cfg,
		publisher: publisher,
		media:     mediaSink,
		app:       app,
	}
	client.AddEventHandler(m.handleEvent)

	if client.Store.ID != nil {
		go func() {
			if err := client.Connect(); err != nil {
				logger.Errorf("conexão automática falhou: %v", err)
			}
		}()
	}

	return m, nil
}

// Close encerra a conexão e libera o SQLite da sessão.
func (m *Manager) Close() {
	if m.client != nil {
		m.client.Disconnect()
	}
	if m.container != nil {
		_ = m.container.Close()
	}
}

// Status devolve o estado atual da sessão.
func (m *Manager) Status(_ context.Context) (*contracts.SessionStatus, error) {
	status := &contracts.SessionStatus{
		Connected: m.client.IsConnected(),
		LoggedIn:  m.client.IsLoggedIn(),
	}
	if m.client.Store.ID != nil {
		status.JID = m.client.Store.ID.String()
	}
	status.PushName = m.client.Store.PushName
	status.Platform = m.client.Store.Platform

	if status.Connected && !m.client.LastSuccessfulConnect.IsZero() {
		status.UptimeSeconds = int64(time.Since(m.client.LastSuccessfulConnect).Seconds())
	} else {
		status.UptimeSeconds = int64(time.Since(m.startedAt).Seconds())
	}
	return status, nil
}

// StartPairing abre o canal de QR e conecta o cliente para pareamento. É
// idempotente enquanto um pareamento estiver em andamento.
func (m *Manager) StartPairing(ctx context.Context) error {
	if m.client.Store.ID != nil && m.client.IsLoggedIn() {
		return ErrAlreadyPaired
	}

	m.pairMu.Lock()
	defer m.pairMu.Unlock()

	if m.pair != nil {
		return nil
	}

	qrChan, err := m.client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("abrir canal de QR: %w", err)
	}

	state := &pairingState{done: make(chan struct{})}
	m.pair = state

	go func() {
		for item := range qrChan {
			switch item.Event {
			case whatsmeow.QRChannelEventCode:
				resp := contracts.QRCodeResponse{Event: "code", Code: item.Code}
				expires := time.Now().Add(pairingTimeout)
				resp.ExpiresAt = &expires
				if png, err := qrcode.Encode(item.Code, qrcode.Medium, 256); err == nil {
					resp.PNGBase64 = base64.StdEncoding.EncodeToString(png)
				}
				state.set(resp)
			case "success":
				state.set(contracts.QRCodeResponse{Event: "success"})
				state.finish(nil)
				return
			case "timeout":
				state.set(contracts.QRCodeResponse{Event: "timeout"})
				state.finish(ErrPairingTimeout)
				return
			default:
				state.finish(fmt.Errorf("evento de QR: %s", item.Event))
				return
			}
		}
		state.finish(ErrPairingTimeout)
	}()

	if !m.client.IsConnected() {
		go func() {
			if err := m.client.Connect(); err != nil {
				state.finish(fmt.Errorf("conectar para pareamento: %w", err))
			}
		}()
	}

	return nil
}

// GetQR devolve o QR Code mais recente do processo de pareamento ativo.
func (m *Manager) GetQR(_ context.Context) (*contracts.QRCodeResponse, error) {
	m.pairMu.Lock()
	state := m.pair
	m.pairMu.Unlock()

	if state == nil {
		return nil, ErrPairingInactive
	}

	resp, hasCode := state.snapshot()
	if !hasCode {
		return &contracts.QRCodeResponse{Event: "timeout"}, nil
	}
	return &resp, nil
}

// PairPhone solicita o código de 8 dígitos. O cliente precisa estar conectado
// e o primeiro QR já deve ter chegado, conforme a documentação do upstream.
func (m *Manager) PairPhone(ctx context.Context, phone string) (*contracts.PairCodeResponse, error) {
	if m.client.Store.ID != nil && m.client.IsLoggedIn() {
		return nil, ErrAlreadyPaired
	}
	if err := m.StartPairing(ctx); err != nil {
		return nil, err
	}

	m.pairMu.Lock()
	state := m.pair
	m.pairMu.Unlock()

	deadline := time.NewTimer(pairingTimeout)
	defer deadline.Stop()

	for {
		if _, hasCode := state.snapshot(); hasCode {
			break
		}
		select {
		case <-state.done:
			if state.err != nil {
				return nil, state.err
			}
			return nil, errors.New("pareamento concluído antes de gerar o código")
		case <-deadline.C:
			return nil, ErrPairingTimeout
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pairPollInterval):
		}
	}

	code, err := m.client.PairPhone(ctx, phone, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		return nil, fmt.Errorf("gerar código de pareamento: %w", err)
	}

	return &contracts.PairCodeResponse{Code: code, ExpiresAt: time.Now().Add(pairCodeTTL)}, nil
}

// Reconnect derruba e restabelece o WebSocket sem deslogar.
func (m *Manager) Reconnect(ctx context.Context) error {
	if m.client.Store.ID == nil {
		return errors.New("dispositivo não pareado")
	}
	m.client.Disconnect()
	if err := m.client.ConnectContext(ctx); err != nil {
		return fmt.Errorf("reconectar: %w", err)
	}
	return nil
}

// Logout encerra a sessão no WhatsApp e limpa o pareamento local.
func (m *Manager) Logout(ctx context.Context) error {
	if m.client.Store.ID == nil {
		return errors.New("dispositivo não pareado")
	}
	if err := m.client.Logout(ctx); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	m.pairMu.Lock()
	m.pair = nil
	m.pairMu.Unlock()
	return nil
}
