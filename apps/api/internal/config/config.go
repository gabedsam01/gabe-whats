// Package config carrega e valida a configuração do daemon da API a partir do
// ambiente. Toda variável tem um valor padrão seguro para uso pessoal local; as
// únicas obrigatórias são a chave de API e as DSNs dos bancos.
package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultHTTPAddr    = "127.0.0.1:8080"
	defaultSQLiteDSN   = "file:./data/session.db?_foreign_keys=on"
	defaultAppDSN      = "file:./data/app.db"
	defaultMediaDir    = "./data/media"
	defaultMediaURLTTL = 15 * time.Minute
	defaultLogLevel    = "info"
	defaultOpenAPI     = "docs/openapi.yaml"
	placeholderAPIKey  = "troque-por-sua-chave-secreta"

	defaultHumanCharsPerSecond = 6.0
	defaultHumanMinGap         = 1200 * time.Millisecond
	defaultHumanMaxGap         = 3500 * time.Millisecond
	defaultHumanMaxTyping      = 8 * time.Second
)

// Webhook reúne a configuração de entrega de eventos aos agentes.
type Webhook struct {
	URL               string
	Secret            string
	IgnoreFromMe      bool
	AllowJIDs         []string
	AutoDownloadMedia bool
	IncludeBase64     bool
}

// Human controla o ritmo de envio: digitação proporcional ao texto, intervalo
// irregular por conversa e espera entre tentativas.
type Human struct {
	Enabled        bool
	CharsPerSecond float64
	MinGap         time.Duration
	MaxGap         time.Duration
	MaxTyping      time.Duration
}

// Config é a configuração efetiva do daemon.
type Config struct {
	HTTPAddr    string
	APIKey      string
	SQLiteDSN   string
	AppDSN      string
	MediaDir    string
	MediaURLTTL time.Duration
	LogLevel    string
	OpenAPIPath string
	Webhook     Webhook
	Human       Human
}

// Load lê o ambiente (aplicando .env, quando existir) e devolve a configuração
// validada. O arquivo .env é opcional: variáveis já exportadas têm precedência.
func Load() (*Config, error) {
	// O erro é intencionalmente ignorado: .env é opcional e o ambiente manda.
	_ = godotenv.Load()

	ttl, err := durationEnv("GABE_WHATS_MEDIA_URL_TTL", defaultMediaURLTTL)
	if err != nil {
		return nil, err
	}

	ignoreFromMe, err := boolEnv("GABE_WHATS_WEBHOOK_IGNORE_FROM_ME", true)
	if err != nil {
		return nil, err
	}

	autoDownload, err := boolEnv("GABE_WHATS_AUTO_DOWNLOAD_MEDIA", true)
	if err != nil {
		return nil, err
	}

	includeBase64, err := boolEnv("GABE_WHATS_WEBHOOK_INCLUDE_BASE64", false)
	if err != nil {
		return nil, err
	}

	humanize, err := boolEnv("GABE_WHATS_HUMANIZE", true)
	if err != nil {
		return nil, err
	}

	typingCPS, err := floatEnv("GABE_WHATS_TYPING_CPS", defaultHumanCharsPerSecond)
	if err != nil {
		return nil, err
	}

	minGap, err := durationEnv("GABE_WHATS_SEND_GAP_MIN", defaultHumanMinGap)
	if err != nil {
		return nil, err
	}

	maxGap, err := durationEnv("GABE_WHATS_SEND_GAP_MAX", defaultHumanMaxGap)
	if err != nil {
		return nil, err
	}

	maxTyping, err := durationEnv("GABE_WHATS_TYPING_MAX", defaultHumanMaxTyping)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		HTTPAddr:    stringEnv("GABE_WHATS_HTTP_ADDR", defaultHTTPAddr),
		APIKey:      strings.TrimSpace(os.Getenv("GABE_WHATS_API_KEY")),
		SQLiteDSN:   stringEnv("GABE_WHATS_SQLITE_DSN", defaultSQLiteDSN),
		AppDSN:      stringEnv("GABE_WHATS_APP_DSN", defaultAppDSN),
		MediaDir:    stringEnv("GABE_WHATS_MEDIA_DIR", defaultMediaDir),
		MediaURLTTL: ttl,
		LogLevel:    stringEnv("GABE_WHATS_LOG_LEVEL", defaultLogLevel),
		OpenAPIPath: stringEnv("GABE_WHATS_OPENAPI_PATH", defaultOpenAPI),
		Webhook: Webhook{
			URL:               strings.TrimSpace(os.Getenv("GABE_WHATS_WEBHOOK_URL")),
			Secret:            os.Getenv("GABE_WHATS_WEBHOOK_SECRET"),
			IgnoreFromMe:      ignoreFromMe,
			AllowJIDs:         listEnv("GABE_WHATS_WEBHOOK_ALLOW_JIDS"),
			AutoDownloadMedia: autoDownload,
			IncludeBase64:     includeBase64,
		},
		Human: Human{
			Enabled:        humanize,
			CharsPerSecond: typingCPS,
			MinGap:         minGap,
			MaxGap:         maxGap,
			MaxTyping:      maxTyping,
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate garante os invariantes mínimos para o daemon subir.
func (c *Config) Validate() error {
	var problems []string

	if c.APIKey == "" {
		problems = append(problems, "GABE_WHATS_API_KEY não definida")
	} else if c.APIKey == placeholderAPIKey {
		problems = append(problems, "GABE_WHATS_API_KEY ainda é o valor de exemplo; defina uma chave real")
	} else if len(c.APIKey) < 16 {
		problems = append(problems, "GABE_WHATS_API_KEY deve ter ao menos 16 caracteres")
	}

	if strings.TrimSpace(c.SQLiteDSN) == "" {
		problems = append(problems, "GABE_WHATS_SQLITE_DSN vazia")
	}
	if strings.TrimSpace(c.AppDSN) == "" {
		problems = append(problems, "GABE_WHATS_APP_DSN vazia")
	}
	if c.SQLiteDSN == c.AppDSN {
		problems = append(problems, "GABE_WHATS_APP_DSN deve ser diferente de GABE_WHATS_SQLITE_DSN (o session.db é exclusivo do whatsmeow)")
	}
	if c.MediaURLTTL <= 0 {
		problems = append(problems, "GABE_WHATS_MEDIA_URL_TTL deve ser positiva")
	}
	if strings.TrimSpace(c.HTTPAddr) == "" {
		problems = append(problems, "GABE_WHATS_HTTP_ADDR vazia")
	}
	if c.Human.Enabled {
		if c.Human.CharsPerSecond <= 0 {
			problems = append(problems, "GABE_WHATS_TYPING_CPS deve ser positiva")
		}
		if c.Human.MinGap < 0 || c.Human.MaxGap < c.Human.MinGap {
			problems = append(problems, "GABE_WHATS_SEND_GAP_MAX deve ser maior ou igual a GABE_WHATS_SEND_GAP_MIN")
		}
		if c.Human.MaxTyping <= 0 {
			problems = append(problems, "GABE_WHATS_TYPING_MAX deve ser positiva")
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("configuração inválida: %s", strings.Join(problems, "; "))
	}
	return nil
}

// IsLoopback informa se o endereço HTTP escuta apenas na máquina local.
func (c *Config) IsLoopback() bool {
	host := c.HTTPAddr
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		host = host[:idx]
	}
	host = strings.Trim(host, "[]")
	switch host {
	case "", "127.0.0.1", "::1", "localhost":
		return true
	default:
		return false
	}
}

// BaseURL devolve a URL base para links públicos (mídia e SSE), trocando
// endereços curinga por loopback.
func (c *Config) BaseURL() string {
	host, port, err := net.SplitHostPort(c.HTTPAddr)
	if err != nil {
		return "http://" + c.HTTPAddr
	}
	switch host {
	case "", "0.0.0.0", "::":
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func stringEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func boolEnv(key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s: valor booleano inválido %q: %w", key, raw, err)
	}
	return v, nil
}

func floatEnv(key string, fallback float64) (float64, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: valor numérico inválido %q: %w", key, raw, err)
	}
	return v, nil
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: duração inválida %q: %w", key, raw, err)
	}
	return v, nil
}

func listEnv(key string) []string {
	raw := os.Getenv(key)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// ErrMissingAPIKey é devolvida quando a chave obrigatória não foi configurada.
var ErrMissingAPIKey = errors.New("GABE_WHATS_API_KEY não definida")
