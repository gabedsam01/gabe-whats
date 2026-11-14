package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gabedsam01/gabe-whats/apps/api/internal/appstore"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/config"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/events"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/media"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/sse"

	_ "github.com/mattn/go-sqlite3"
)

const testAPIKey = "chave-de-teste-com-32-caracteres-ok"

type s3Fixture struct {
	server  *Server
	app     *appstore.Store
	media   *media.Store
	tickets *sse.Tickets
	hub     *sse.Hub
}

func newS3Fixture(t *testing.T, mediaTTL time.Duration) *s3Fixture {
	t.Helper()

	dir := t.TempDir()
	app, err := appstore.Open(context.Background(), "file:"+filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatalf("appstore: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })

	mediaStore, err := media.New(app, filepath.Join(dir, "media"), "http://127.0.0.1:8080", mediaTTL, nil)
	if err != nil {
		t.Fatalf("media: %v", err)
	}

	hub := sse.NewHub(nil)
	tickets := sse.NewTickets(time.Minute)
	bus := events.NewBus(events.NewRecent(20), hub, nil, nil, nil)

	cfg := &config.Config{
		HTTPAddr:    "127.0.0.1:8080",
		APIKey:      testAPIKey,
		SQLiteDSN:   "file:./data/session.db",
		AppDSN:      "file:./data/app.db",
		MediaURLTTL: mediaTTL,
		LogLevel:    "error",
		OpenAPIPath: "../../../../docs/openapi.yaml",
		Webhook: config.Webhook{
			URL:          "http://127.0.0.1:18789/hook",
			IgnoreFromMe: true,
		},
	}

	server := New(cfg, Deps{Bus: bus, Hub: hub, Tickets: tickets, App: app, Media: mediaStore})
	return &s3Fixture{server: server, app: app, media: mediaStore, tickets: tickets, hub: hub}
}

func TestTicketAndStream(t *testing.T) {
	fixture := newS3Fixture(t, time.Minute)
	httpServer := httptest.NewServer(fixture.server.Handler())
	defer httpServer.Close()

	// Sem chave: rejeita.
	response, err := http.Post(httpServer.URL+"/api/v1/events/ticket", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status sem chave = %d; esperado 401", response.StatusCode)
	}

	// Com chave: emite ticket.
	request, _ := http.NewRequest(http.MethodPost, httpServer.URL+"/api/v1/events/ticket", nil)
	request.Header.Set("X-API-Key", testAPIKey)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status do ticket = %d", response.StatusCode)
	}

	var ticket struct {
		Ticket    string    `json:"ticket"`
		ExpiresAt time.Time `json:"expires_at"`
		StreamURL string    `json:"stream_url"`
	}
	if err := json.NewDecoder(response.Body).Decode(&ticket); err != nil {
		t.Fatalf("decodificar ticket: %v", err)
	}
	if ticket.Ticket == "" || !strings.Contains(ticket.StreamURL, ticket.Ticket) {
		t.Fatalf("ticket inválido: %+v", ticket)
	}

	// Abre o stream com o ticket.
	streamResponse, err := http.Get(httpServer.URL + "/api/v1/events/stream?ticket=" + ticket.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	if streamResponse.StatusCode != http.StatusOK {
		t.Fatalf("status do stream = %d", streamResponse.StatusCode)
	}
	if ct := streamResponse.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q", ct)
	}
	buffer := make([]byte, 32)
	_, _ = streamResponse.Body.Read(buffer)
	_ = streamResponse.Body.Close()

	// Ticket é de uso único.
	reused, err := http.Get(httpServer.URL + "/api/v1/events/stream?ticket=" + ticket.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	_ = reused.Body.Close()
	if reused.StatusCode != http.StatusUnauthorized {
		t.Errorf("reuso do ticket = %d; esperado 401", reused.StatusCode)
	}
}

func TestWebhookConfigEndpoints(t *testing.T) {
	fixture := newS3Fixture(t, time.Minute)
	handler := fixture.server.Handler()

	get := func() map[string]any {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks", nil)
		request.Header.Set("X-API-Key", testAPIKey)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET status = %d", recorder.Code)
		}
		var body map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("GET corpo: %v", err)
		}
		return body
	}

	initial := get()
	if initial["url"] != "http://127.0.0.1:18789/hook" {
		t.Errorf("url inicial = %v", initial["url"])
	}
	if initial["secret_set"] != false {
		t.Errorf("secret_set inicial = %v", initial["secret_set"])
	}

	payload := []byte(`{"url":"http://127.0.0.1:9000/agent","secret":"segredo-hmac","ignore_from_me":true,"allow_jids":["5511999999999@s.whatsapp.net"],"auto_download_media":false,"include_base64":true}`)
	putRequest := httptest.NewRequest(http.MethodPut, "/api/v1/webhooks", bytes.NewReader(payload))
	putRequest.Header.Set("X-API-Key", testAPIKey)
	putRequest.Header.Set("Content-Type", "application/json")
	putRecorder := httptest.NewRecorder()
	handler.ServeHTTP(putRecorder, putRequest)
	if putRecorder.Code != http.StatusOK {
		t.Fatalf("PUT status = %d: %s", putRecorder.Code, putRecorder.Body.String())
	}
	if strings.Contains(putRecorder.Body.String(), "segredo-hmac") {
		t.Error("o segredo foi ecoado na resposta")
	}

	updated := get()
	if updated["url"] != "http://127.0.0.1:9000/agent" {
		t.Errorf("url após PUT = %v", updated["url"])
	}
	if updated["secret_set"] != true {
		t.Errorf("secret_set = %v", updated["secret_set"])
	}
	if updated["include_base64"] != true {
		t.Errorf("include_base64 = %v", updated["include_base64"])
	}

	// PUT sem segredo preserva o segredo existente.
	second := []byte(`{"url":"http://127.0.0.1:9000/agent","ignore_from_me":true,"auto_download_media":true,"include_base64":false}`)
	secondRequest := httptest.NewRequest(http.MethodPut, "/api/v1/webhooks", bytes.NewReader(second))
	secondRequest.Header.Set("X-API-Key", testAPIKey)
	secondRequest.Header.Set("Content-Type", "application/json")
	secondRecorder := httptest.NewRecorder()
	handler.ServeHTTP(secondRecorder, secondRequest)
	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("segundo PUT = %d: %s", secondRecorder.Code, secondRecorder.Body.String())
	}

	settings, err := fixture.app.GetWebhook(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if settings.Secret != "segredo-hmac" {
		t.Errorf("segredo não preservado: %q", settings.Secret)
	}
}

func TestMediaEndpoint(t *testing.T) {
	fixture := newS3Fixture(t, time.Nanosecond)
	handler := fixture.server.Handler()

	request := httptest.NewRequest(http.MethodGet, "/api/v1/media/inexistente", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("mídia inexistente = %d; esperado 404", recorder.Code)
	}

	ref, err := fixture.media.Save(context.Background(), []byte("dados"), "text/plain", "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)

	expired := httptest.NewRequest(http.MethodGet, "/api/v1/media/"+ref.ID, nil)
	expiredRecorder := httptest.NewRecorder()
	handler.ServeHTTP(expiredRecorder, expired)
	if expiredRecorder.Code != http.StatusGone {
		t.Errorf("mídia expirada = %d; esperado 410", expiredRecorder.Code)
	}
}

func TestRecentMessagesEmpty(t *testing.T) {
	fixture := newS3Fixture(t, time.Minute)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/messages/recent", nil)
	request.Header.Set("X-API-Key", testAPIKey)
	recorder := httptest.NewRecorder()
	fixture.server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	body, _ := io.ReadAll(recorder.Body)
	var parsed struct {
		Messages []any `json:"messages"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("corpo: %v (%s)", err, body)
	}
	if parsed.Messages == nil {
		t.Error("messages deveria ser [] e não null")
	}
}
