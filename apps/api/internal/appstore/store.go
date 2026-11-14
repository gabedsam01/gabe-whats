// Package appstore mantém o banco SQLite próprio do daemon (`app.db`), separado
// do `session.db` do whatsmeow. Aqui vivem a configuração de webhook, o registro
// de mídias servidas por URL temporária e, no futuro, o cache de enquetes.
package appstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gabedsam01/gabe-whats/shared/contracts"
)

// Store é o handle do app.db.
type Store struct {
	db *sql.DB
}

// Open abre (criando se necessário) o banco e aplica o schema.
func Open(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("appstore: abrir %q: %w", dsn, err)
	}
	// SQLite local não se beneficia de pool; uma conexão evita "database is locked".
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("appstore: %s: %w", pragma, err)
		}
	}

	store := &Store{db: db}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// Close libera o banco.
func (s *Store) Close() error { return s.db.Close() }

// Ping verifica a saúde do banco.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS webhook_config (
	id                INTEGER PRIMARY KEY CHECK (id = 1),
	url               TEXT    NOT NULL DEFAULT '',
	secret            TEXT    NOT NULL DEFAULT '',
	ignore_from_me    INTEGER NOT NULL DEFAULT 1,
	allow_jids        TEXT    NOT NULL DEFAULT '[]',
	auto_download_media INTEGER NOT NULL DEFAULT 1,
	include_base64    INTEGER NOT NULL DEFAULT 0,
	updated_at        TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS media (
	id         TEXT PRIMARY KEY,
	mime_type  TEXT    NOT NULL,
	filename   TEXT    NOT NULL DEFAULT '',
	size_bytes INTEGER NOT NULL,
	path       TEXT    NOT NULL,
	created_at TEXT    NOT NULL,
	expires_at TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_media_expires ON media(expires_at);

CREATE TABLE IF NOT EXISTS poll_cache (
	poll_message_id         TEXT PRIMARY KEY,
	chat_jid                TEXT NOT NULL,
	question                TEXT NOT NULL,
	options                 TEXT NOT NULL,
	selectable_option_count INTEGER NOT NULL DEFAULT 1,
	poll_sender_jid         TEXT NOT NULL DEFAULT '',
	created_at              TEXT NOT NULL
);
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("appstore: aplicar schema: %w", err)
	}
	return nil
}

// ErrNotFound indica registro inexistente.
var ErrNotFound = errors.New("appstore: registro não encontrado")

// WebhookSettings é a configuração persistida (inclui o segredo).
type WebhookSettings struct {
	URL               string
	Secret            string
	IgnoreFromMe      bool
	AllowJIDs         []string
	AutoDownloadMedia bool
	IncludeBase64     bool
	UpdatedAt         time.Time
}

// GetWebhook devolve a configuração persistida ou ErrNotFound.
func (s *Store) GetWebhook(ctx context.Context) (*WebhookSettings, error) {
	const query = `
SELECT url, secret, ignore_from_me, allow_jids, auto_download_media, include_base64, updated_at
FROM webhook_config WHERE id = 1`

	var (
		settings  WebhookSettings
		allowRaw  string
		updatedAt string
	)
	err := s.db.QueryRowContext(ctx, query).Scan(
		&settings.URL, &settings.Secret, &settings.IgnoreFromMe, &allowRaw,
		&settings.AutoDownloadMedia, &settings.IncludeBase64, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("appstore: ler webhook: %w", err)
	}

	if allowRaw != "" {
		if err := json.Unmarshal([]byte(allowRaw), &settings.AllowJIDs); err != nil {
			return nil, fmt.Errorf("appstore: allow_jids inválido: %w", err)
		}
	}
	if settings.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt); err != nil {
		settings.UpdatedAt = time.Time{}
	}
	return &settings, nil
}

// SaveWebhook grava (upsert) a configuração.
func (s *Store) SaveWebhook(ctx context.Context, settings WebhookSettings) error {
	allowRaw, err := json.Marshal(settings.AllowJIDs)
	if err != nil {
		return fmt.Errorf("appstore: serializar allow_jids: %w", err)
	}
	if settings.AllowJIDs == nil {
		allowRaw = []byte("[]")
	}

	const query = `
INSERT INTO webhook_config (id, url, secret, ignore_from_me, allow_jids, auto_download_media, include_base64, updated_at)
VALUES (1, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	url = excluded.url,
	secret = excluded.secret,
	ignore_from_me = excluded.ignore_from_me,
	allow_jids = excluded.allow_jids,
	auto_download_media = excluded.auto_download_media,
	include_base64 = excluded.include_base64,
	updated_at = excluded.updated_at`

	_, err = s.db.ExecContext(ctx, query,
		settings.URL, settings.Secret, settings.IgnoreFromMe, string(allowRaw),
		settings.AutoDownloadMedia, settings.IncludeBase64, time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("appstore: gravar webhook: %w", err)
	}
	return nil
}

// PublicSettings devolve a configuração sem o segredo, no formato do contrato.
func (w *WebhookSettings) PublicSettings() contracts.WebhookConfigResponse {
	allow := w.AllowJIDs
	if allow == nil {
		allow = []string{}
	}
	return contracts.WebhookConfigResponse{
		URL:               w.URL,
		IgnoreFromMe:      w.IgnoreFromMe,
		AllowJIDs:         allow,
		AutoDownloadMedia: w.AutoDownloadMedia,
		IncludeBase64:     w.IncludeBase64,
		SecretSet:         w.Secret != "",
	}
}

// MediaRecord é uma mídia registrada para download temporário.
type MediaRecord struct {
	ID        string
	MimeType  string
	Filename  string
	SizeBytes int64
	Path      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// SaveMedia registra a mídia e devolve o registro completo.
func (s *Store) SaveMedia(ctx context.Context, record MediaRecord) error {
	const query = `
INSERT INTO media (id, mime_type, filename, size_bytes, path, created_at, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		record.ID, record.MimeType, record.Filename, record.SizeBytes, record.Path,
		record.CreatedAt.UTC().Format(time.RFC3339Nano), record.ExpiresAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("appstore: gravar mídia: %w", err)
	}
	return nil
}

// GetMedia busca a mídia pelo ID.
func (s *Store) GetMedia(ctx context.Context, id string) (*MediaRecord, error) {
	const query = `
SELECT id, mime_type, filename, size_bytes, path, created_at, expires_at
FROM media WHERE id = ?`

	var (
		record    MediaRecord
		createdAt string
		expiresAt string
	)
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&record.ID, &record.MimeType, &record.Filename, &record.SizeBytes, &record.Path, &createdAt, &expiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("appstore: ler mídia: %w", err)
	}
	record.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	record.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expiresAt)
	return &record, nil
}

// PurgeExpiredMedia remove registros vencidos e devolve os caminhos a apagar.
func (s *Store) PurgeExpiredMedia(ctx context.Context, now time.Time) ([]string, error) {
	const selectQuery = `SELECT path FROM media WHERE expires_at < ?`
	rows, err := s.db.QueryContext(ctx, selectQuery, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, fmt.Errorf("appstore: selecionar mídias vencidas: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}

	if _, err := s.db.ExecContext(ctx, `DELETE FROM media WHERE expires_at < ?`, now.UTC().Format(time.RFC3339Nano)); err != nil {
		return nil, fmt.Errorf("appstore: apagar mídias vencidas: %w", err)
	}
	return paths, nil
}

// PollRecord é a criação de uma enquete guardada para resolver votos.
type PollRecord struct {
	MessageID             string
	ChatJID               string
	Question              string
	Options               []string
	SelectableOptionCount int
	PollSenderJID         string
	CreatedAt             time.Time
}

// SavePoll grava (upsert) a criação da enquete.
func (s *Store) SavePoll(ctx context.Context, record PollRecord) error {
	options, err := json.Marshal(record.Options)
	if err != nil {
		return fmt.Errorf("appstore: serializar opções da enquete: %w", err)
	}

	const query = `
INSERT INTO poll_cache (poll_message_id, chat_jid, question, options, selectable_option_count, poll_sender_jid, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(poll_message_id) DO UPDATE SET
	question = excluded.question,
	options = excluded.options,
	selectable_option_count = excluded.selectable_option_count`

	_, err = s.db.ExecContext(ctx, query,
		record.MessageID, record.ChatJID, record.Question, string(options),
		record.SelectableOptionCount, record.PollSenderJID, time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("appstore: gravar enquete: %w", err)
	}
	return nil
}

// GetPoll recupera a enquete pelo ID da mensagem de criação.
func (s *Store) GetPoll(ctx context.Context, messageID string) (*PollRecord, error) {
	const query = `
SELECT poll_message_id, chat_jid, question, options, selectable_option_count, poll_sender_jid, created_at
FROM poll_cache WHERE poll_message_id = ?`

	var (
		record  PollRecord
		options string
		created string
	)
	err := s.db.QueryRowContext(ctx, query, messageID).Scan(
		&record.MessageID, &record.ChatJID, &record.Question, &options,
		&record.SelectableOptionCount, &record.PollSenderJID, &created,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("appstore: ler enquete: %w", err)
	}
	if err := json.Unmarshal([]byte(options), &record.Options); err != nil {
		return nil, fmt.Errorf("appstore: opções inválidas: %w", err)
	}
	record.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return &record, nil
}
