package appstore

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "app.db")
	store, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestWebhookConfigRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if _, err := store.GetWebhook(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("esperado ErrNotFound, obtido %v", err)
	}

	settings := WebhookSettings{
		URL:               "http://127.0.0.1:18789/webhooks/whatsapp",
		Secret:            "segredo",
		IgnoreFromMe:      true,
		AllowJIDs:         []string{"5511999999999@s.whatsapp.net"},
		AutoDownloadMedia: true,
		IncludeBase64:     false,
	}
	if err := store.SaveWebhook(ctx, settings); err != nil {
		t.Fatalf("SaveWebhook: %v", err)
	}

	got, err := store.GetWebhook(ctx)
	if err != nil {
		t.Fatalf("GetWebhook: %v", err)
	}
	if got.URL != settings.URL || got.Secret != settings.Secret {
		t.Errorf("valores divergentes: %+v", got)
	}
	if len(got.AllowJIDs) != 1 || got.AllowJIDs[0] != settings.AllowJIDs[0] {
		t.Errorf("AllowJIDs = %v", got.AllowJIDs)
	}

	public := got.PublicSettings()
	if !public.SecretSet {
		t.Error("SecretSet deveria ser true")
	}
	if public.URL != settings.URL {
		t.Errorf("URL pública = %q", public.URL)
	}
}

func TestMediaLifecycle(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	record := MediaRecord{
		ID:        "abc",
		MimeType:  "image/jpeg",
		Filename:  "foto.jpg",
		SizeBytes: 42,
		Path:      "/tmp/abc",
		CreatedAt: now,
		ExpiresAt: now.Add(15 * time.Minute),
	}
	if err := store.SaveMedia(ctx, record); err != nil {
		t.Fatalf("SaveMedia: %v", err)
	}

	got, err := store.GetMedia(ctx, "abc")
	if err != nil {
		t.Fatalf("GetMedia: %v", err)
	}
	if got.MimeType != record.MimeType || got.SizeBytes != record.SizeBytes {
		t.Errorf("registro divergente: %+v", got)
	}

	if _, err := store.GetMedia(ctx, "inexistente"); !errors.Is(err, ErrNotFound) {
		t.Errorf("esperado ErrNotFound, obtido %v", err)
	}

	paths, err := store.PurgeExpiredMedia(ctx, now.Add(16*time.Minute))
	if err != nil {
		t.Fatalf("PurgeExpiredMedia: %v", err)
	}
	if len(paths) != 1 || paths[0] != record.Path {
		t.Errorf("paths = %v", paths)
	}
	if _, err := store.GetMedia(ctx, "abc"); !errors.Is(err, ErrNotFound) {
		t.Errorf("registro deveria ter sido removido, obtido %v", err)
	}
}
func TestPollCacheRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	record := PollRecord{
		MessageID:             "poll-1",
		ChatJID:               "5511999999999@s.whatsapp.net",
		Question:              "Qual o melhor horário?",
		Options:               []string{"Manhã", "Tarde", "Noite"},
		SelectableOptionCount: 1,
		PollSenderJID:         "5511888888888@s.whatsapp.net",
	}
	if err := store.SavePoll(ctx, record); err != nil {
		t.Fatalf("SavePoll: %v", err)
	}

	got, err := store.GetPoll(ctx, "poll-1")
	if err != nil {
		t.Fatalf("GetPoll: %v", err)
	}
	if got.Question != record.Question {
		t.Errorf("Question = %q", got.Question)
	}
	if len(got.Options) != 3 || got.Options[2] != "Noite" {
		t.Errorf("Options = %v", got.Options)
	}
	if got.SelectableOptionCount != 1 {
		t.Errorf("SelectableOptionCount = %d", got.SelectableOptionCount)
	}

	// Upsert atualiza a pergunta sem duplicar.
	record.Question = "Pergunta corrigida"
	if err := store.SavePoll(ctx, record); err != nil {
		t.Fatalf("SavePoll upsert: %v", err)
	}
	got, _ = store.GetPoll(ctx, "poll-1")
	if got.Question != "Pergunta corrigida" {
		t.Errorf("upsert não atualizou: %q", got.Question)
	}

	if _, err := store.GetPoll(ctx, "naoexiste"); !errors.Is(err, ErrNotFound) {
		t.Errorf("esperado ErrNotFound, obtido %v", err)
	}
}
