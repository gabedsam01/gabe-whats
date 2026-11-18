package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gabedsam01/gabe-whats/apps/api/internal/appstore"

	_ "github.com/mattn/go-sqlite3"
)

func newTestStore(t *testing.T, ttl time.Duration) (*Store, *appstore.Store) {
	t.Helper()
	dir := t.TempDir()
	app, err := appstore.Open(context.Background(), "file:"+filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatalf("appstore: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })

	store, err := New(app, filepath.Join(dir, "media"), "http://127.0.0.1:8080/", ttl, nil)
	if err != nil {
		t.Fatalf("media.New: %v", err)
	}
	return store, app
}

func TestSaveAndGet(t *testing.T) {
	store, _ := newTestStore(t, 15*time.Minute)
	ctx := context.Background()

	ref, err := store.Save(ctx, []byte("conteudo"), "image/jpeg", "foto.jpg")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if ref.ID == "" {
		t.Fatal("ID vazio")
	}
	if ref.SizeBytes != int64(len("conteudo")) {
		t.Errorf("SizeBytes = %d", ref.SizeBytes)
	}
	if ref.DownloadURL != "http://127.0.0.1:8080/api/v1/media/"+ref.ID {
		t.Errorf("DownloadURL = %q", ref.DownloadURL)
	}
	if time.Until(ref.ExpiresAt) <= 0 {
		t.Error("expiração no passado")
	}

	record, err := store.Get(ctx, ref.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	data, err := os.ReadFile(record.Path)
	if err != nil {
		t.Fatalf("arquivo não existe: %v", err)
	}
	if string(data) != "conteudo" {
		t.Errorf("conteúdo = %q", data)
	}
}

func TestExpiredMedia(t *testing.T) {
	store, _ := newTestStore(t, time.Nanosecond)
	ctx := context.Background()

	ref, err := store.Save(ctx, []byte("x"), "text/plain", "a.txt")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	time.Sleep(2 * time.Millisecond)

	if _, err := store.Get(ctx, ref.ID); !errors.Is(err, ErrExpired) {
		t.Fatalf("esperado ErrExpired, obtido %v", err)
	}

	store.Purge(ctx)
	if _, err := os.Stat(filepath.Join(store.dir, ref.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("arquivo deveria ter sido apagado, obtido %v", err)
	}
}

func TestUnknownMedia(t *testing.T) {
	store, _ := newTestStore(t, time.Minute)
	if _, err := store.Get(context.Background(), "naoexiste"); !errors.Is(err, appstore.ErrNotFound) {
		t.Fatalf("esperado ErrNotFound, obtido %v", err)
	}
}
