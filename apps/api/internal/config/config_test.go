package config

import (
	"strings"
	"testing"
	"time"
)

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GABE_WHATS_API_KEY", "chave-de-teste-com-32-caracteres-ok")
	t.Setenv("GABE_WHATS_HTTP_ADDR", "")
	t.Setenv("GABE_WHATS_SQLITE_DSN", "")
	t.Setenv("GABE_WHATS_APP_DSN", "")
	t.Setenv("GABE_WHATS_MEDIA_URL_TTL", "")
	t.Setenv("GABE_WHATS_WEBHOOK_IGNORE_FROM_ME", "")
	t.Setenv("GABE_WHATS_AUTO_DOWNLOAD_MEDIA", "")
	t.Setenv("GABE_WHATS_WEBHOOK_INCLUDE_BASE64", "")
}

func TestLoadAppliesDefaults(t *testing.T) {
	setValidEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() devolveu erro inesperado: %v", err)
	}

	if cfg.HTTPAddr != defaultHTTPAddr {
		t.Errorf("HTTPAddr = %q; esperado %q", cfg.HTTPAddr, defaultHTTPAddr)
	}
	if cfg.SQLiteDSN != defaultSQLiteDSN {
		t.Errorf("SQLiteDSN = %q; esperado %q", cfg.SQLiteDSN, defaultSQLiteDSN)
	}
	if cfg.AppDSN != defaultAppDSN {
		t.Errorf("AppDSN = %q; esperado %q", cfg.AppDSN, defaultAppDSN)
	}
	if cfg.MediaURLTTL != defaultMediaURLTTL {
		t.Errorf("MediaURLTTL = %v; esperado %v", cfg.MediaURLTTL, defaultMediaURLTTL)
	}
	if !cfg.Webhook.IgnoreFromMe {
		t.Error("IgnoreFromMe deveria ser true por padrão")
	}
	if !cfg.Webhook.AutoDownloadMedia {
		t.Error("AutoDownloadMedia deveria ser true por padrão")
	}
}

func TestLoadValidation(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T)
		wantSub string
	}{
		{
			name:    "chave ausente",
			setup:   func(t *testing.T) { t.Setenv("GABE_WHATS_API_KEY", "") },
			wantSub: "GABE_WHATS_API_KEY não definida",
		},
		{
			name:    "chave de exemplo",
			setup:   func(t *testing.T) { t.Setenv("GABE_WHATS_API_KEY", placeholderAPIKey) },
			wantSub: "valor de exemplo",
		},
		{
			name:    "chave curta",
			setup:   func(t *testing.T) { t.Setenv("GABE_WHATS_API_KEY", "curta") },
			wantSub: "ao menos 16 caracteres",
		},
		{
			name: "dsn igual",
			setup: func(t *testing.T) {
				t.Setenv("GABE_WHATS_SQLITE_DSN", "file:./data/mesmo.db")
				t.Setenv("GABE_WHATS_APP_DSN", "file:./data/mesmo.db")
			},
			wantSub: "deve ser diferente",
		},
		{
			name:    "ttl invalida",
			setup:   func(t *testing.T) { t.Setenv("GABE_WHATS_MEDIA_URL_TTL", "quinze minutos") },
			wantSub: "duração inválida",
		},
		{
			name:    "booleano invalido",
			setup:   func(t *testing.T) { t.Setenv("GABE_WHATS_WEBHOOK_IGNORE_FROM_ME", "talvez") },
			wantSub: "booleano inválido",
		},
		{
			name:    "ttl zero",
			setup:   func(t *testing.T) { t.Setenv("GABE_WHATS_MEDIA_URL_TTL", "0s") },
			wantSub: "deve ser positiva",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setValidEnv(t)
			tc.setup(t)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() deveria falhar para %q", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("erro %q não contém %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestLoadParsesConfiguredValues(t *testing.T) {
	setValidEnv(t)
	t.Setenv("GABE_WHATS_MEDIA_URL_TTL", "45m")
	t.Setenv("GABE_WHATS_WEBHOOK_ALLOW_JIDS", " 5511999999999@s.whatsapp.net , 5511888888888@s.whatsapp.net ")
	t.Setenv("GABE_WHATS_WEBHOOK_IGNORE_FROM_ME", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() devolveu erro: %v", err)
	}
	if cfg.MediaURLTTL != 45*time.Minute {
		t.Errorf("MediaURLTTL = %v; esperado 45m", cfg.MediaURLTTL)
	}
	if len(cfg.Webhook.AllowJIDs) != 2 {
		t.Fatalf("AllowJIDs = %v; esperado 2 entradas", cfg.Webhook.AllowJIDs)
	}
	if cfg.Webhook.IgnoreFromMe {
		t.Error("IgnoreFromMe deveria ser false quando configurado")
	}
}

func TestIsLoopback(t *testing.T) {
	tests := map[string]bool{
		"127.0.0.1:8080":    true,
		"localhost:8080":    true,
		"[::1]:8080":        true,
		":8080":             true,
		"0.0.0.0:8080":      false,
		"192.168.1.10:8080": false,
	}
	for addr, want := range tests {
		cfg := &Config{HTTPAddr: addr}
		if got := cfg.IsLoopback(); got != want {
			t.Errorf("IsLoopback(%q) = %v; esperado %v", addr, got, want)
		}
	}
}
