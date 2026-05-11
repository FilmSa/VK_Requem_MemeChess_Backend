package inventory

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"meme_chess/internal/auth"
)

type HTTP struct {
	Svc  *Service
	Auth *auth.Service
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (h *HTTP) GetCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Svc == nil {
		writeError(w, http.StatusInternalServerError, "inventory service unavailable")
		return
	}
	items, err := h.Svc.GetCatalog(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load catalog")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *HTTP) GetMyInventory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Svc == nil || h.Auth == nil {
		writeError(w, http.StatusInternalServerError, "inventory service unavailable")
		return
	}

	u, err := h.Auth.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	inv, err := h.Svc.GetInventory(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load inventory")
		return
	}

	writeJSON(w, http.StatusOK, inv)
}

type setSelectionRequest struct {
	PieceSkinSlug *string  `json:"piece_skin_slug"`
	BoardSkinSlug *string  `json:"board_skin_slug"`
	EmoteSlugs    []string `json:"emote_slugs"`
	StickerSlugs  []string `json:"sticker_slugs,omitempty"`
}

func (h *HTTP) PutMySelection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Svc == nil || h.Auth == nil {
		writeError(w, http.StatusInternalServerError, "inventory service unavailable")
		return
	}

	u, err := h.Auth.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req setSelectionRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
	}

	sel, err := h.Svc.SetSelection(r.Context(), u.ID, Selection{
		PieceSkinSlug: req.PieceSkinSlug,
		BoardSkinSlug: req.BoardSkinSlug,
		EmoteSlugs:    resolveEmoteSlugs(req),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrTooManyEmotes), errors.Is(err, ErrDuplicateEmotes):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrItemNotOwned):
			writeError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, ErrItemNotFound), errors.Is(err, ErrInvalidItemType):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "failed to update selection")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"selected": sel,
	})
}

func resolveEmoteSlugs(req setSelectionRequest) []string {
	if len(req.EmoteSlugs) > 0 {
		return req.EmoteSlugs
	}
	return req.StickerSlugs
}

func RegisterRoutes(mux *http.ServeMux, h *HTTP) {
	if mux == nil || h == nil {
		return
	}

	mux.HandleFunc("/api/v1/inventory/catalog", h.GetCatalog)
	mux.HandleFunc("/api/v1/inventory/me", h.GetMyInventory)
	mux.HandleFunc("/api/v1/inventory/me/selection", h.PutMySelection)
}

func NormalizeSlugPtr(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}
