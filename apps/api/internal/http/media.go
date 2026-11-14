package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gabedsam01/gabe-whats/apps/api/internal/appstore"
	"github.com/gabedsam01/gabe-whats/apps/api/internal/media"
	"github.com/go-chi/chi/v5"
)

// registerMedia publica o download temporário de mídias.
func (s *Server) registerMedia() {
	s.router.Get("/api/v1/media/{id}", s.handleMedia)
}

// handleMedia serve o arquivo enquanto a URL temporária for válida.
func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	if s.deps.Media == nil {
		writeProblem(w, http.StatusServiceUnavailable, "storage_unavailable", "armazenamento de mídia não inicializado")
		return
	}

	record, err := s.deps.Media.Get(r.Context(), chi.URLParam(r, "id"))
	switch {
	case errors.Is(err, appstore.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "not_found", "mídia não encontrada")
		return
	case errors.Is(err, media.ErrExpired):
		writeProblem(w, http.StatusGone, "media_expired", "URL temporária expirada; a mídia foi removida")
		return
	case err != nil:
		writeProblem(w, http.StatusInternalServerError, "internal_error", "não foi possível consultar a mídia")
		return
	}

	file, err := os.Open(record.Path)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "arquivo da mídia indisponível")
		return
	}
	defer func() { _ = file.Close() }()

	remaining := int(time.Until(record.ExpiresAt).Seconds())
	if remaining < 0 {
		remaining = 0
	}

	w.Header().Set("Content-Type", record.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", record.Filename))
	w.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d", remaining))
	w.Header().Set("ETag", fmt.Sprintf("%q", record.ID))

	http.ServeContent(w, r, record.Filename, record.CreatedAt, file)
}
