package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/config"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{
		HTTPAddr:    "127.0.0.1:0",
		APIKey:      "chave-de-teste-com-32-caracteres-ok",
		SQLiteDSN:   "file:./data/session.db",
		AppDSN:      "file:./data/app.db",
		MediaURLTTL: 15 * time.Minute,
		LogLevel:    "error",
		OpenAPIPath: "../../../../docs/openapi.yaml",
	}
	s := New(cfg, Deps{})
	if len(s.specJSON) == 0 {
		t.Fatal("a especificação OpenAPI não foi carregada do disco")
	}
	return s
}

func doRequest(t *testing.T, s *Server, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	s := newTestServer(t)

	rec := doRequest(t, s, http.MethodGet, "/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; esperado 200", rec.Code)
	}

	var body struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("corpo inválido: %v (%s)", err, rec.Body.String())
	}
	if body.Status != "ok" {
		t.Errorf("status = %q; esperado ok", body.Status)
	}
	if body.Version == "" {
		t.Error("version vazia")
	}
}

func TestAuthRejectsMissingAndWrongKey(t *testing.T) {
	s := newTestServer(t)

	cases := map[string]map[string]string{
		"sem chave":     nil,
		"chave errada":  {"X-API-Key": "chave-errada"},
		"bearer errado": {"Authorization": "Bearer chave-errada"},
	}

	for name, headers := range cases {
		t.Run(name, func(t *testing.T) {
			rec := doRequest(t, s, http.MethodGet, "/api/v1/session/status", headers)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d; esperado 401", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, contentTypeProblem) {
				t.Errorf("Content-Type = %q; esperado %s", ct, contentTypeProblem)
			}
			var problem struct {
				Status int    `json:"status"`
				Code   string `json:"code"`
				Detail string `json:"detail"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
				t.Fatalf("corpo não é JSON: %v", err)
			}
			if problem.Status != http.StatusUnauthorized || problem.Code != "unauthorized" {
				t.Errorf("problem = %+v; esperado status 401 e code unauthorized", problem)
			}
		})
	}
}

func TestDocsAndSpec(t *testing.T) {
	s := newTestServer(t)

	rec := doRequest(t, s, http.MethodGet, "/docs", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Scalar") {
		t.Errorf("/docs falhou: status=%d", rec.Code)
	}

	rec = doRequest(t, s, http.MethodGet, "/openapi.yaml", nil)
	if rec.Code != http.StatusOK || !strings.HasPrefix(strings.TrimSpace(rec.Body.String()), "openapi:") {
		t.Errorf("/openapi.yaml inválido: status=%d", rec.Code)
	}

	rec = doRequest(t, s, http.MethodGet, "/openapi.json", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("/openapi.json status=%d", rec.Code)
	}
	var spec struct {
		OpenAPI string         `json:"openapi"`
		Paths   map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("/openapi.json não é JSON válido: %v", err)
	}
	if spec.OpenAPI != "3.1.0" {
		t.Errorf("openapi = %q; esperado 3.1.0", spec.OpenAPI)
	}
	if len(spec.Paths) != 32 {
		t.Errorf("paths = %d; esperado 32", len(spec.Paths))
	}
}

func TestHumaRoutesArePlanned(t *testing.T) {
	s := newTestServer(t)

	planned := make(map[string]bool, len(PlannedRoutes))
	for _, route := range PlannedRoutes {
		planned[strings.ToUpper(route.Method)+" "+route.Path] = true
	}

	skip := map[string]bool{
		"GET /docs":         true,
		"GET /openapi.json": true,
		"GET /openapi.yaml": true,
	}

	registered := registeredOperations(s.api)
	if len(registered) == 0 {
		t.Fatal("nenhuma operação registrada no Huma")
	}

	for _, key := range registered {
		if skip[key] {
			continue
		}
		if !planned[key] {
			t.Errorf("rota registrada no Huma mas ausente de PlannedRoutes: %s", key)
		}
	}
}

func registeredOperations(api huma.API) []string {
	var out []string
	for path, item := range api.OpenAPI().Paths {
		if item == nil {
			continue
		}
		operations := map[string]*huma.Operation{
			http.MethodGet:    item.Get,
			http.MethodPost:   item.Post,
			http.MethodPut:    item.Put,
			http.MethodPatch:  item.Patch,
			http.MethodDelete: item.Delete,
			http.MethodHead:   item.Head,
		}
		for method, op := range operations {
			if op == nil {
				continue
			}
			out = append(out, method+" "+path)
		}
	}
	return out
}
