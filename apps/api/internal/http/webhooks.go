package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/appstore"
	"github.com/gabedsam01/gabe-whats/shared/contracts"
)

// registerWebhooks publica a configuração de webhook.
func (s *Server) registerWebhooks() {
	type settingsOutput struct {
		Body *contracts.WebhookConfigResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/api/v1/webhooks",
		OperationID: "getWebhookConfig",
		Summary:     "Consulta a configuração atual do webhook",
		Tags:        []string{"Webhooks & Events"},
	}, func(ctx context.Context, _ *struct{}) (*settingsOutput, error) {
		settings, err := s.webhookSettings(ctx)
		if err != nil {
			return nil, newProblem(http.StatusInternalServerError, "internal_error", "não foi possível ler a configuração de webhook")
		}
		public := settings.PublicSettings()
		return &settingsOutput{Body: &public}, nil
	})

	type settingsInput struct {
		Body *contracts.WebhookConfig
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodPut,
		Path:        "/api/v1/webhooks",
		OperationID: "updateWebhookConfig",
		Summary:     "Atualiza URL, segredo e filtros do webhook",
		Tags:        []string{"Webhooks & Events"},
	}, func(ctx context.Context, in *settingsInput) (*settingsOutput, error) {
		if in.Body == nil {
			return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed", "corpo da requisição ausente")
		}
		if in.Body.URL != "" {
			parsed, err := url.Parse(in.Body.URL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
				return nil, newProblem(http.StatusUnprocessableEntity, "validation_failed",
					"url inválida: informe uma URI HTTP(S) absoluta",
					fieldError("url", "esperado http(s)://host/caminho"))
			}
		}
		if s.deps.App == nil {
			return nil, newProblem(http.StatusServiceUnavailable, "storage_unavailable", "app.db não inicializado")
		}

		current, err := s.webhookSettings(ctx)
		if err != nil {
			return nil, newProblem(http.StatusInternalServerError, "internal_error", "não foi possível ler a configuração atual")
		}

		// O segredo é write-only: campo vazio preserva o valor existente.
		secret := in.Body.Secret
		if secret == "" {
			secret = current.Secret
		}

		settings := appstore.WebhookSettings{
			URL:               in.Body.URL,
			Secret:            secret,
			IgnoreFromMe:      in.Body.IgnoreFromMe,
			AllowJIDs:         in.Body.AllowJIDs,
			AutoDownloadMedia: in.Body.AutoDownloadMedia,
			IncludeBase64:     in.Body.IncludeBase64,
		}
		if err := s.deps.App.SaveWebhook(ctx, settings); err != nil {
			return nil, newProblem(http.StatusInternalServerError, "internal_error", fmt.Sprintf("não foi possível salvar: %v", err))
		}

		public := settings.PublicSettings()
		return &settingsOutput{Body: &public}, nil
	})
}

// webhookSettings resolve a configuração: app.db primeiro, ambiente como padrão.
func (s *Server) webhookSettings(ctx context.Context) (*appstore.WebhookSettings, error) {
	if s.deps.App != nil {
		settings, err := s.deps.App.GetWebhook(ctx)
		if err == nil {
			return settings, nil
		}
		if !errors.Is(err, appstore.ErrNotFound) {
			return nil, err
		}
	}

	return &appstore.WebhookSettings{
		URL:               s.cfg.Webhook.URL,
		Secret:            s.cfg.Webhook.Secret,
		IgnoreFromMe:      s.cfg.Webhook.IgnoreFromMe,
		AllowJIDs:         s.cfg.Webhook.AllowJIDs,
		AutoDownloadMedia: s.cfg.Webhook.AutoDownloadMedia,
		IncludeBase64:     s.cfg.Webhook.IncludeBase64,
	}, nil
}
