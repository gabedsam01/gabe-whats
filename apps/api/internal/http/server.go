package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/appstore"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/config"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/events"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/media"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/sse"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/whatsapp"
	"github.com/gabedsam01/gabe-whats/shared/contracts"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gopkg.in/yaml.v3"
)

// Version é a versão da API exposta em /health e na documentação.
const Version = "0.3.0-s3"

// publicPaths são as rotas que dispensam autenticação.
var publicPaths = map[string]bool{
	"/health":       true,
	"/docs":         true,
	"/openapi.json": true,
	"/openapi.yaml": true,
}

// Deps reúne as dependências externas do servidor HTTP.
type Deps struct {
	WhatsApp *whatsapp.Manager
	Bus      *events.Bus
	Hub      *sse.Hub
	Tickets  *sse.Tickets
	App      *appstore.Store
	Media    *media.Store
}

// Server monta o roteador, o pipeline Huma e a documentação da API.
type Server struct {
	cfg  *config.Config
	deps Deps

	router chi.Router
	api    huma.API

	specJSON []byte
	specYAML []byte
}

// New constrói o servidor HTTP com todas as rotas registradas.
func New(cfg *config.Config, deps Deps) *Server {
	installProblemErrors()

	s := &Server{cfg: cfg, deps: deps}
	s.loadSpec()

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(s.recoverer)
	router.Use(s.authenticate)

	humaCfg := huma.DefaultConfig("gabe-whats REST API", Version)
	humaCfg.OpenAPIPath = ""
	humaCfg.DocsPath = ""
	humaCfg.SchemasPath = ""
	humaCfg.Info.Description = "API REST pessoal sobre go.mau.fi/whatsmeow para envio e recebimento de mensagens, mídias, grupos e eventos."
	humaCfg.Info.Contact = nil
	// Sem o link transformer do Huma: o corpo deve casar exatamente com os
	// contratos de shared/contracts, sem campos $schema injetados. O default
	// instala esse transformer por CreateHooks, então é preciso limpar ambos.
	humaCfg.Transformers = nil
	humaCfg.CreateHooks = nil

	s.router = router
	s.api = humachi.New(router, humaCfg)

	s.registerHealth()
	s.registerSession()
	s.registerMessages()
	s.registerWebhooks()
	s.registerEventsTicket()
	s.registerStream()
	s.registerMedia()
	s.registerDocs()

	return s
}

// Handler devolve o http.Handler raiz com o pipeline completo.
func (s *Server) Handler() http.Handler { return s.router }

// API expõe o registro Huma para testes de paridade com PlannedRoutes.
func (s *Server) API() huma.API { return s.api }

// registerDocs serve a documentação e a especificação publicada.
func (s *Server) registerDocs() {
	s.router.Get("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(s.jsonSpec())
	})
	s.router.Get("/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(s.yamlSpec())
	})
	s.router.Get("/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(scalarDocsHTML))
	})
}

// registerHealth publica o healthcheck.
func (s *Server) registerHealth() {
	type output struct {
		Body *contracts.HealthResponse
	}

	huma.Register(s.api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/health",
		OperationID: "healthCheck",
		Summary:     "Healthcheck da API",
		Tags:        []string{"Health"},
	}, func(_ context.Context, _ *struct{}) (*output, error) {
		return &output{Body: &contracts.HealthResponse{
			Status:  "ok",
			Version: Version,
		}}, nil
	})
}

// authenticate exige X-API-Key/Bearer em /api/v1/*. O stream SSE aceita um
// ticket efêmero na query porque EventSource não envia headers.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			next.ServeHTTP(w, r)
			return
		}

		// A URL de mídia é uma capacidade: o ID é aleatório de 128 bits e expira
		// em 15 minutos, então o próprio link é o segredo (como URL pré-assinada).
		if strings.HasPrefix(r.URL.Path, "/api/v1/media/") {
			next.ServeHTTP(w, r)
			return
		}

		if r.URL.Path == "/api/v1/events/stream" {
			if ticket := r.URL.Query().Get("ticket"); ticket != "" {
				if s.deps.Tickets != nil && s.deps.Tickets.Redeem(ticket) {
					next.ServeHTTP(w, r)
					return
				}
				writeProblem(w, http.StatusUnauthorized, "unauthorized", "ticket inválido, expirado ou já utilizado")
				return
			}
		}

		key := r.Header.Get("X-API-Key")
		if key == "" {
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				key = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if subtle.ConstantTimeCompare([]byte(key), []byte(s.cfg.APIKey)) != 1 {
			writeProblem(w, http.StatusUnauthorized, "unauthorized", "chave de API ausente ou inválida")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recoverer converte panic em Problem 500 sem derrubar o processo.
func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic no handler", "path", r.URL.Path, "panic", rec)
				writeProblem(w, http.StatusInternalServerError, "internal_error", "erro interno inesperado")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// loadSpec lê a especificação curada em docs/openapi.yaml e prepara as versões
// JSON e YAML servidas pela API. Em caso de falha, a API usa a especificação
// gerada pelo Huma como fallback, registrando o aviso.
func (s *Server) loadSpec() {
	if strings.TrimSpace(s.cfg.OpenAPIPath) == "" {
		slog.Warn("GABE_WHATS_OPENAPI_PATH vazio; servindo a especificação gerada pelo Huma")
		return
	}

	raw, err := os.ReadFile(s.cfg.OpenAPIPath)
	if err != nil {
		slog.Warn("não foi possível ler a especificação OpenAPI; usando a gerada", "path", s.cfg.OpenAPIPath, "erro", err)
		return
	}

	var doc any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		slog.Warn("especificação OpenAPI inválida; usando a gerada", "path", s.cfg.OpenAPIPath, "erro", err)
		return
	}

	encoded, err := json.Marshal(doc)
	if err != nil {
		slog.Warn("não foi possível converter a especificação para JSON; usando a gerada", "erro", err)
		return
	}

	s.specJSON = encoded
	s.specYAML = raw
}

// jsonSpec devolve a especificação em JSON, com fallback para a do Huma.
func (s *Server) jsonSpec() []byte {
	if len(s.specJSON) > 0 {
		return s.specJSON
	}
	encoded, err := json.Marshal(s.api.OpenAPI())
	if err != nil {
		return []byte("{}")
	}
	return encoded
}

// yamlSpec devolve a especificação em YAML, com fallback para a do Huma.
func (s *Server) yamlSpec() []byte {
	if len(s.specYAML) > 0 {
		return s.specYAML
	}
	encoded, err := yaml.Marshal(s.api.OpenAPI())
	if err != nil {
		return []byte("openapi: 3.1.0\n")
	}
	return encoded
}

// scalarDocsHTML é a página de documentação interativa.
const scalarDocsHTML = `<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>gabe-whats — API REST</title>
  <style>body{margin:0;font-family:ui-sans-serif,system-ui,sans-serif}</style>
</head>
<body>
  <div id="app"></div>
  <noscript>
    <p style="padding:1rem">
      Habilite JavaScript para a documentação interativa, ou baixe a especificação em
      <a href="/openapi.json">/openapi.json</a> e <a href="/openapi.yaml">/openapi.yaml</a>.
    </p>
  </noscript>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  <script>
    Scalar.createApiReference('#app', { url: '/openapi.json' });
  </script>
</body>
</html>`
