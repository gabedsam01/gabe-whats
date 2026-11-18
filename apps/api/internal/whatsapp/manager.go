// Package whatsapp encapsula o cliente go.mau.fi/whatsmeow atrás de uma API
// pequena e estável consumida pelos handlers HTTP. Nenhum tipo do whatsmeow
// vaza para fora deste pacote: a fronteira é o pacote shared/contracts.
package whatsapp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gabedsam01/gabe-whats/apps/api/internal/appstore"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/config"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/human"
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

	human *human.Profile
	pacer *human.Pacer

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
	cancel  context.CancelFunc
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

func (p *pairingState) isDone() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

func (p *pairingState) finish(err error) {
	p.once.Do(func() {
		p.mu.Lock()
		p.err = err
		cancel := p.cancel
		p.mu.Unlock()
		if cancel != nil {
			cancel()
		}
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
	profile := human.NewProfile(cfg.Human.Enabled, cfg.Human.CharsPerSecond, cfg.Human.MinGap, cfg.Human.MaxGap, cfg.Human.MaxTyping, 0)
	m := &Manager{
		client:    client,
		container: container,
		startedAt: time.Now(),
		cfg:       cfg,
		publisher: publisher,
		media:     mediaSink,
		app:       app,
		human:     profile,
		pacer:     human.NewPacer(profile),
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
// idempotente enquanto houver um pareamento ativo conectado, e reinicia
// automaticamente caso o canal anterior tenha expirado ou desconectado.
func (m *Manager) StartPairing(_ context.Context) error {
	if m.client.Store.ID != nil && m.client.IsLoggedIn() {
		return ErrAlreadyPaired
	}

	m.pairMu.Lock()
	defer m.pairMu.Unlock()

	if m.pair != nil {
		if !m.pair.isDone() && m.client.IsConnected() {
			return nil
		}
		slog.Warn("descartando estado de pareamento expirado ou desconectado",
			"done", m.pair.isDone(),
			"connected", m.client.IsConnected(),
		)
		m.pair.finish(ErrPairingTimeout)
		m.pair = nil
	}

	// O whatsmeow exige que GetQRChannel seja chamado antes de Connect().
	if m.client.IsConnected() {
		slog.Info("desconectando WebSocket antes de iniciar novo canal de QR")
		m.client.Disconnect()
	}

	// O canal de QR precisa sobreviver além da requisição HTTP que o iniciou.
	pairCtx, cancel := context.WithTimeout(context.Background(), pairCodeTTL+30*time.Second)
	qrChan, err := m.client.GetQRChannel(pairCtx)
	if err != nil {
		cancel()
		slog.Error("falha ao abrir canal de QR", "erro", err)
		return fmt.Errorf("abrir canal de QR: %w", err)
	}

	state := &pairingState{
		done:   make(chan struct{}),
		cancel: cancel,
	}
	m.pair = state
	slog.Info("iniciando sessão de pareamento")

	go func() {
		defer func() {
			m.pairMu.Lock()
			if m.pair == state {
				m.pair = nil
			}
			m.pairMu.Unlock()
		}()

		for item := range qrChan {
			switch item.Event {
			case whatsmeow.QRChannelEventCode:
				slog.Info("QR Code gerado", "tamanho", len(item.Code))
				resp := contracts.QRCodeResponse{Event: "code", Code: item.Code}
				expires := time.Now().Add(pairingTimeout)
				resp.ExpiresAt = &expires
				if png, err := qrcode.Encode(item.Code, qrcode.Medium, 256); err == nil {
					resp.PNGBase64 = base64.StdEncoding.EncodeToString(png)
				}
				state.set(resp)
			case "success":
				slog.Info("pareamento concluído com sucesso")
				state.set(contracts.QRCodeResponse{Event: "success"})
				state.finish(nil)
				return
			case "timeout":
				slog.Warn("timeout do canal de QR")
				state.set(contracts.QRCodeResponse{Event: "timeout"})
				state.finish(ErrPairingTimeout)
				return
			default:
				slog.Warn("canal de QR encerrado com evento", "evento", item.Event)
				state.finish(fmt.Errorf("evento de QR: %s", item.Event))
				return
			}
		}
		slog.Warn("canal de QR encerrado sem confirmação")
		state.finish(ErrPairingTimeout)
	}()

	if !m.client.IsConnected() {
		go func() {
			if err := m.client.Connect(); err != nil {
				slog.Error("falha ao conectar para pareamento", "erro", err)
				state.finish(fmt.Errorf("conectar para pareamento: %w", err))
			}
		}()
	}

	return nil
}

// GetQR devolve o QR Code mais recente do processo de pareamento ativo,
// aguardando o primeiro código chegar durante o handshake inicial.
func (m *Manager) GetQR(ctx context.Context) (*contracts.QRCodeResponse, error) {
	m.pairMu.Lock()
	state := m.pair
	m.pairMu.Unlock()

	if state == nil {
		return nil, ErrPairingInactive
	}

	if err := m.waitForQRCode(ctx, state); err != nil {
		if errors.Is(err, ErrPairingTimeout) {
			return &contracts.QRCodeResponse{Event: "timeout"}, nil
		}
		return nil, err
	}

	resp, hasCode := state.snapshot()
	if !hasCode {
		return &contracts.QRCodeResponse{Event: "timeout"}, nil
	}
	return &resp, nil
}

// PairPhone solicita o código de 8 dígitos. O cliente precisa estar conectado
// e o primeiro QR já deve ter chegado, conforme a documentação do upstream.
// Se a primeira tentativa falhar (por exemplo, canal antigo expirando),
// reinicia a conexão uma vez e tenta novamente.
func (m *Manager) PairPhone(ctx context.Context, phone string) (*contracts.PairCodeResponse, error) {
	slog.Info("solicitando código de pareamento por telefone", "phone", phone)

	if m.client.Store.ID != nil && m.client.IsLoggedIn() {
		return nil, ErrAlreadyPaired
	}

	code, err := m.waitAndPairPhone(ctx, phone)
	if err != nil && !errors.Is(err, ErrAlreadyPaired) && ctx.Err() == nil {
		slog.Warn("primeira tentativa de PairPhone falhou; reiniciando conexão de pareamento",
			"phone", phone,
			"erro", err,
		)
		m.resetPairing(err)
		code, err = m.waitAndPairPhone(ctx, phone)
	}
	if err != nil {
		slog.Error("falha ao gerar código de pareamento", "phone", phone, "erro", err)
		return nil, err
	}

	slog.Info("código de pareamento gerado com sucesso", "phone", phone)
	return &contracts.PairCodeResponse{Code: code, ExpiresAt: time.Now().Add(pairCodeTTL)}, nil
}

func (m *Manager) waitAndPairPhone(ctx context.Context, phone string) (string, error) {
	if err := m.StartPairing(ctx); err != nil {
		return "", err
	}

	m.pairMu.Lock()
	state := m.pair
	m.pairMu.Unlock()

	if state == nil {
		return "", ErrPairingInactive
	}
	if err := m.waitForQRCode(ctx, state); err != nil {
		return "", err
	}

	code, err := m.client.PairPhone(ctx, phone, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		return "", fmt.Errorf("gerar código de pareamento: %w", err)
	}
	return code, nil
}

func (m *Manager) waitForQRCode(ctx context.Context, state *pairingState) error {
	deadline := time.NewTimer(pairingTimeout)
	defer deadline.Stop()

	for {
		if _, hasCode := state.snapshot(); hasCode {
			return nil
		}
		select {
		case <-state.done:
			if state.err != nil {
				return state.err
			}
			return errors.New("pareamento concluído antes de gerar o código")
		case <-deadline.C:
			return ErrPairingTimeout
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pairPollInterval):
		}
	}
}

func (m *Manager) resetPairing(cause error) {
	m.pairMu.Lock()
	if m.pair != nil {
		m.pair.finish(cause)
		m.pair = nil
	}
	m.pairMu.Unlock()
	if m.client.IsConnected() {
		m.client.Disconnect()
	}
}

// Reconnect derruba e restabelece o WebSocket sem deslogar.
//
// A reconexão roda em background: usar o contexto da requisição HTTP faria o
// connect ser cancelado no instante em que a resposta fosse escrita. O retorno
// indica apenas que a reconexão foi agendada.
func (m *Manager) Reconnect(_ context.Context) error {
	if m.client.Store.ID == nil {
		return errors.New("dispositivo não pareado")
	}

	if m.client.IsConnected() {
		// ResetConnection força o auto-reconnect do upstream sem deslogar.
		m.client.ResetConnection()
		return nil
	}

	// Já desconectado: abrir uma conexão nova em background, porque o contexto
	// da requisição HTTP morre junto com a resposta.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := m.client.ConnectContext(ctx); err != nil {
			m.client.Log.Errorf("reconexão falhou: %v", err)
		}
	}()

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
