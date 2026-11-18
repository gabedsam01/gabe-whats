// Package media armazena mídias recebidas e as expõe por URL temporária.
package media

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabedsam01/gabe-whats/apps/api/internal/appstore"
	"github.com/gabedsam01/gabe-whats/shared/contracts"
)

// ErrExpired indica que a URL temporária venceu.
var ErrExpired = errors.New("media: URL temporária expirada")

// Store grava arquivos no disco e metadados no app.db.
type Store struct {
	app     *appstore.Store
	dir     string
	ttl     time.Duration
	baseURL string
	log     *slog.Logger
}

// New prepara o diretório de mídias.
func New(app *appstore.Store, dir, baseURL string, ttl time.Duration, logger *slog.Logger) (*Store, error) {
	if dir == "" {
		return nil, errors.New("media: diretório vazio")
	}
	if ttl <= 0 {
		return nil, errors.New("media: TTL deve ser positivo")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("media: criar diretório %q: %w", dir, err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Store{
		app:     app,
		dir:     dir,
		ttl:     ttl,
		baseURL: strings.TrimSuffix(baseURL, "/"),
		log:     logger,
	}, nil
}

// Save grava o conteúdo e devolve a referência com a URL temporária.
func (s *Store) Save(ctx context.Context, data []byte, mimeType, filename string) (*contracts.MediaRef, error) {
	id, err := newID()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(s.dir, id)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, fmt.Errorf("media: gravar arquivo: %w", err)
	}

	now := time.Now().UTC()
	record := appstore.MediaRecord{
		ID:        id,
		MimeType:  mimeType,
		Filename:  filename,
		SizeBytes: int64(len(data)),
		Path:      path,
		CreatedAt: now,
		ExpiresAt: now.Add(s.ttl),
	}
	if err := s.app.SaveMedia(ctx, record); err != nil {
		_ = os.Remove(path)
		return nil, err
	}

	return &contracts.MediaRef{
		ID:          id,
		MimeType:    mimeType,
		Filename:    filename,
		SizeBytes:   int64(len(data)),
		DownloadURL: s.baseURL + "/api/v1/media/" + id,
		ExpiresAt:   record.ExpiresAt,
	}, nil
}

// Get devolve o registro se ainda estiver válido.
func (s *Store) Get(ctx context.Context, id string) (*appstore.MediaRecord, error) {
	record, err := s.app.GetMedia(ctx, id)
	if err != nil {
		return nil, err
	}
	if time.Now().After(record.ExpiresAt) {
		return record, ErrExpired
	}
	return record, nil
}

// Purge remove arquivos e registros vencidos. É seguro chamar periodicamente.
func (s *Store) Purge(ctx context.Context) {
	paths, err := s.app.PurgeExpiredMedia(ctx, time.Now().UTC())
	if err != nil {
		s.log.Warn("media: falha ao limpar registros vencidos", "erro", err)
		return
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.Warn("media: falha ao apagar arquivo vencido", "path", path, "erro", err)
		}
	}
	if len(paths) > 0 {
		s.log.Debug("media: arquivos vencidos removidos", "quantidade", len(paths))
	}
}

// newID gera um identificador opaco de 16 bytes.
func newID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("media: gerar id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}
