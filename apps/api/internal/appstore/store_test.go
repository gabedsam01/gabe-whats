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
