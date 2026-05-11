package shop

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"meme_chess/internal/auth"
	"meme_chess/internal/user"
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
		writeError(w, http.StatusInternalServerError, "shop service unavailable")
		return
	}

	userID := ""
	authHeader := r.Header.Get("Authorization")
	if strings.TrimSpace(authHeader) != "" {
		if h.Auth == nil {
			writeError(w, http.StatusInternalServerError, "shop auth unavailable")
			return
		}

		u, err := h.Auth.UserFromBearer(r.Context(), authHeader)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		userID = u.ID
	}

	items, err := h.Svc.GetCatalog(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load shop catalog")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type convertRequest struct {
	Amount int64 `json:"amount"`
}

func (h *HTTP) PostConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Svc == nil || h.Auth == nil {
		writeError(w, http.StatusInternalServerError, "shop service unavailable")
		return
	}

	u, err := h.Auth.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req convertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	cur, err := h.Svc.ConvertGameToShop(r.Context(), u.ID, req.Amount)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidAmount):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, user.ErrInsufficientGameCurrency):
			writeError(w, http.StatusConflict, "insufficient game currency")
		default:
			writeError(w, http.StatusInternalServerError, "failed to convert currency")
		}
		return
	}

	writeJSON(w, http.StatusOK, cur)
}

type buyRequest struct {
	Slug string `json:"slug"`
}

func (h *HTTP) PostBuy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Svc == nil || h.Auth == nil {
		writeError(w, http.StatusInternalServerError, "shop service unavailable")
		return
	}

	u, err := h.Auth.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req buyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	cur, err := h.Svc.Buy(r.Context(), u.ID, req.Slug)
	if err != nil {
		switch {
		case errors.Is(err, ErrItemNotForSale):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrItemAlreadyOwned):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, ErrInsufficientShopCurrency):
			writeError(w, http.StatusConflict, "insufficient shop currency")
		default:
			writeError(w, http.StatusInternalServerError, "failed to buy item")
		}
		return
	}

	writeJSON(w, http.StatusOK, cur)
}

func RegisterRoutes(mux *http.ServeMux, h *HTTP) {
	if mux == nil || h == nil {
		return
	}

	mux.HandleFunc("/api/v1/shop/catalog", h.GetCatalog)
	mux.HandleFunc("/api/v1/shop/convert", h.PostConvert)
	mux.HandleFunc("/api/v1/shop/buy", h.PostBuy)
}
