package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabedsam01/gabe-whats/shared/contracts"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return New(server.URL, "chave-teste")
}

func TestStatusSendsAPIKey(t *testing.T) {
	var gotKey string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		_ = json.NewEncoder(w).Encode(contracts.SessionStatus{Connected: true, LoggedIn: true, JID: "5511999999999@s.whatsapp.net"})
	})

	status, err := client.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if gotKey != "chave-teste" {
		t.Errorf("X-API-Key = %q", gotKey)
	}
	if !status.Connected || status.JID == "" {
		t.Errorf("status inesperado: %+v", status)
	}
}

func TestErrorMapping(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(contracts.Problem{
			Status: http.StatusUnprocessableEntity,
			Title:  "Unprocessable Entity",
			Detail: "número inválido",
			Code:   "validation_failed",
		})
	})

	_, err := client.SendText(context.Background(), contracts.SendTextRequest{To: "x", Text: "y"})
	if err == nil {
		t.Fatal("esperava erro")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("erro deveria ser *APIError, obtido %T", err)
	}
	if apiErr.Status != http.StatusUnprocessableEntity || apiErr.Code != "validation_failed" {
		t.Errorf("APIError = %+v", apiErr)
	}
	if !strings.Contains(apiErr.Detail, "número inválido") {
		t.Errorf("detalhe = %q", apiErr.Detail)
	}
}

func TestMessageIDIsEscaped(t *testing.T) {
	var gotURI string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.RequestURI
		_ = json.NewEncoder(w).Encode(contracts.MessageResponse{Status: "revoked"})
	})

	if _, err := client.RevokeMessage(context.Background(), "3EB0/abc", contracts.RevokeMessageRequest{ChatJID: "5511999999999@s.whatsapp.net"}); err != nil {
		t.Fatalf("RevokeMessage: %v", err)
	}
	if !strings.Contains(gotURI, "3EB0%2Fabc") {
		t.Errorf("caminho não escapado na URI: %q", gotURI)
	}
}

func TestRecentQuery(t *testing.T) {
	var gotQuery string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(contracts.RecentMessagesResponse{Messages: []contracts.MessageSummary{}})
	})

	if _, err := client.RecentMessages(context.Background(), 25, "5511999999999@s.whatsapp.net"); err != nil {
		t.Fatalf("RecentMessages: %v", err)
	}
	if !strings.Contains(gotQuery, "limit=25") || !strings.Contains(gotQuery, "chat_jid=") {
		t.Errorf("query = %q", gotQuery)
	}
}

func TestLeaveGroupWithoutBody(t *testing.T) {
	var method, path string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		_ = json.NewEncoder(w).Encode(contracts.AckResponse{OK: true})
	})

	if _, err := client.LeaveGroup(context.Background(), "123@g.us"); err != nil {
		t.Fatalf("LeaveGroup: %v", err)
	}
	if method != http.MethodPost || !strings.Contains(path, "/groups/123@g.us/leave") {
		t.Errorf("requisição = %s %s", method, path)
	}
}
