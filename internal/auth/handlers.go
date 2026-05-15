package auth

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"meme_chess/internal/user"
)

type Handlers struct {
	Service    *Service
	UploadsDir string
}

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string     `json:"token"`
	User  userPublic `json:"user"`
}

type currencyResponse struct {
	ShopFunds int64 `json:"shop_funds"`
	GameFunds int64 `json:"game_funds"`
}

type userPublic struct {
	ID        string     `json:"id"`
	Username  string     `json:"username"`
	Email     *string    `json:"email,omitempty"`
	AvatarURL *string    `json:"avatar_url,omitempty"`
	ShopFunds int64      `json:"shop_funds"`
	GameFunds int64      `json:"game_funds"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

const (
	maxAvatarUploadSize = 5 << 20
	avatarUploadField   = "avatar"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	token, u, err := h.Service.Register(r.Context(), RegisterInput{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidUsername):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrWeakPassword):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrDuplicateUser):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "registration failed")
		}
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{
		Token: token,
		User:  buildUserPublic(u),
	})
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	token, u, err := h.Service.Login(r.Context(), LoginInput{
		Login:    req.Login,
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}
	if u == nil {
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{
		Token: token,
		User:  buildUserPublic(u),
	})
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		u, err := h.Service.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
		if err != nil {
			switch {
			case errors.Is(err, ErrMissingToken):
				writeError(w, http.StatusUnauthorized, err.Error())
			case errors.Is(err, ErrInvalidToken):
				writeError(w, http.StatusUnauthorized, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "failed to load user")
			}
			return
		}
		writeJSON(w, http.StatusOK, buildUserPublic(u))
	case http.MethodPatch:
		u, err := h.Service.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
		if err != nil {
			switch {
			case errors.Is(err, ErrMissingToken):
				writeError(w, http.StatusUnauthorized, err.Error())
			case errors.Is(err, ErrInvalidToken):
				writeError(w, http.StatusUnauthorized, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "failed to load user")
			}
			return
		}

		body, err := readBodyLimit(r, 1<<20)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		patch, err := parseProfilePatch(body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		updated, err := h.Service.UpdateProfile(r.Context(), u.ID, patch)
		if err != nil {
			switch {
			case errors.Is(err, ErrNoProfileChanges):
				writeError(w, http.StatusBadRequest, err.Error())
			case errors.Is(err, ErrProfileEmailConflict), errors.Is(err, ErrProfileAvatarURLConflict):
				writeError(w, http.StatusBadRequest, err.Error())
			case errors.Is(err, ErrInvalidUsername):
				writeError(w, http.StatusBadRequest, err.Error())
			case errors.Is(err, ErrDuplicateUser):
				writeError(w, http.StatusConflict, err.Error())
			case errors.Is(err, ErrAvatarURLTooLong):
				writeError(w, http.StatusBadRequest, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "profile update failed")
			}
			return
		}
		shouldRemovePreviousAvatar := patch.ClearAvatarURL
		if patch.AvatarURLSet && u.AvatarURL != nil {
			shouldRemovePreviousAvatar = strings.TrimSpace(*u.AvatarURL) != strings.TrimSpace(patch.AvatarURL)
		}
		if shouldRemovePreviousAvatar && u.AvatarURL != nil {
			h.removeManagedAvatar(*u.AvatarURL)
		}
		writeJSON(w, http.StatusOK, buildUserPublic(updated))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func readBodyLimit(r *http.Request, max int64) ([]byte, error) {
	limited := io.LimitReader(r.Body, max+1)
	b, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, errors.New("body too large")
	}
	return b, nil
}

func parseProfilePatch(body []byte) (ProfilePatch, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return ProfilePatch{}, errors.New("invalid json")
	}
	if len(raw) == 0 {
		return ProfilePatch{}, errors.New("empty json object")
	}

	var p ProfilePatch
	for k := range raw {
		switch k {
		case "username", "email", "avatar_url", "clear_email", "clear_avatar_url":
		default:
			return ProfilePatch{}, fmt.Errorf("unknown field: %s", k)
		}
	}

	if b, ok := raw["username"]; ok {
		if string(b) == "null" {
			return ProfilePatch{}, errors.New("username cannot be null")
		}
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return ProfilePatch{}, errors.New("invalid username")
		}
		p.UsernameSet = true
		p.Username = s
	}
	if b, ok := raw["email"]; ok {
		if string(b) == "null" {
			// omit: do not change stored email
		} else {
			var s string
			if err := json.Unmarshal(b, &s); err != nil {
				return ProfilePatch{}, errors.New("invalid email")
			}
			if e := strings.TrimSpace(s); e != "" {
				p.EmailSet = true
				p.Email = e
			}
		}
	}
	if b, ok := raw["avatar_url"]; ok {
		if string(b) == "null" {
			// omit: do not change stored avatar_url
		} else {
			var s string
			if err := json.Unmarshal(b, &s); err != nil {
				return ProfilePatch{}, errors.New("invalid avatar_url")
			}
			if a := strings.TrimSpace(s); a != "" {
				p.AvatarURLSet = true
				p.AvatarURL = a
			}
		}
	}
	if b, ok := raw["clear_email"]; ok {
		var v bool
		if err := json.Unmarshal(b, &v); err != nil {
			return ProfilePatch{}, errors.New("invalid clear_email")
		}
		if v {
			p.ClearEmail = true
		}
	}
	if b, ok := raw["clear_avatar_url"]; ok {
		var v bool
		if err := json.Unmarshal(b, &v); err != nil {
			return ProfilePatch{}, errors.New("invalid clear_avatar_url")
		}
		if v {
			p.ClearAvatarURL = true
		}
	}
	if p.ClearEmail && p.EmailSet {
		return ProfilePatch{}, ErrProfileEmailConflict
	}
	if p.ClearAvatarURL && p.AvatarURLSet {
		return ProfilePatch{}, ErrProfileAvatarURLConflict
	}
	if !p.UsernameSet && !p.EmailSet && !p.AvatarURLSet && !p.ClearEmail && !p.ClearAvatarURL {
		return ProfilePatch{}, ErrNoProfileChanges
	}
	return p, nil
}

func (h *Handlers) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	u, err := h.Service.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		switch {
		case errors.Is(err, ErrMissingToken):
			writeError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, ErrInvalidToken):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "failed to load user")
		}
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarUploadSize+1024)
	if err := r.ParseMultipartForm(maxAvatarUploadSize); err != nil {
		if isBodyTooLarge(err) {
			writeError(w, http.StatusBadRequest, "avatar file is too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, _, err := r.FormFile(avatarUploadField)
	if err != nil {
		writeError(w, http.StatusBadRequest, "avatar file is required")
		return
	}
	defer file.Close()

	avatarBytes, extension, err := readAvatarUpload(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	publicAvatarURL, err := h.storeAvatarFile(u.ID, extension, avatarBytes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save avatar")
		return
	}

	updated, err := h.Service.UpdateProfile(r.Context(), u.ID, ProfilePatch{
		AvatarURLSet: true,
		AvatarURL:    publicAvatarURL,
	})
	if err != nil {
		h.removeManagedAvatar(publicAvatarURL)
		switch {
		case errors.Is(err, ErrAvatarURLTooLong):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "profile update failed")
		}
		return
	}

	if u.AvatarURL != nil && strings.TrimSpace(*u.AvatarURL) != strings.TrimSpace(publicAvatarURL) {
		h.removeManagedAvatar(*u.AvatarURL)
	}

	writeJSON(w, http.StatusOK, buildUserPublic(updated))
}

func isBodyTooLarge(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "request body too large") || strings.Contains(msg, "http: request body too large")
}

func readAvatarUpload(file io.Reader) ([]byte, string, error) {
	avatarBytes, err := io.ReadAll(io.LimitReader(file, maxAvatarUploadSize+1))
	if err != nil {
		return nil, "", errors.New("failed to read avatar file")
	}
	if len(avatarBytes) == 0 {
		return nil, "", errors.New("avatar file is empty")
	}
	if len(avatarBytes) > maxAvatarUploadSize {
		return nil, "", errors.New("avatar file is too large")
	}

	switch contentType := http.DetectContentType(avatarBytes); contentType {
	case "image/png":
		return avatarBytes, ".png", nil
	case "image/jpeg":
		return avatarBytes, ".jpg", nil
	case "image/webp":
		return avatarBytes, ".webp", nil
	case "image/gif":
		return avatarBytes, ".gif", nil
	default:
		return nil, "", errors.New("unsupported avatar format")
	}
}

func (h *Handlers) avatarUploadsDir() string {
	baseDir := strings.TrimSpace(h.UploadsDir)
	if baseDir == "" {
		baseDir = "uploads"
	}
	return filepath.Join(baseDir, "avatars")
}

func (h *Handlers) storeAvatarFile(userID, extension string, avatarBytes []byte) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", errors.New("invalid user id")
	}

	if err := os.MkdirAll(h.avatarUploadsDir(), 0o755); err != nil {
		return "", err
	}

	filename, err := newAvatarFilename(userID, extension)
	if err != nil {
		return "", err
	}

	baseDir := strings.TrimSpace(h.UploadsDir)
	if baseDir == "" {
		baseDir = "uploads"
	}

	relativePath := filepath.Join("avatars", filename)
	absolutePath := filepath.Join(baseDir, relativePath)
	if err := os.WriteFile(absolutePath, avatarBytes, 0o644); err != nil {
		return "", err
	}

	return "/uploads/" + filepath.ToSlash(relativePath), nil
}

func newAvatarFilename(userID, extension string) (string, error) {
	var randomBytes [6]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return "", err
	}

	safeUserID := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_").Replace(strings.TrimSpace(userID))
	if safeUserID == "" {
		safeUserID = "user"
	}

	return fmt.Sprintf("%s-%d-%x%s", safeUserID, time.Now().UnixNano(), randomBytes[:], extension), nil
}

func (h *Handlers) managedAvatarPath(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/uploads/") {
		return ""
	}

	relativePath := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(trimmed, "/uploads/")))
	if relativePath == "." || strings.HasPrefix(relativePath, "..") || filepath.IsAbs(relativePath) {
		return ""
	}

	baseDir := strings.TrimSpace(h.UploadsDir)
	if baseDir == "" {
		baseDir = "uploads"
	}

	return filepath.Join(baseDir, relativePath)
}

func (h *Handlers) removeManagedAvatar(rawURL string) {
	managedPath := h.managedAvatarPath(rawURL)
	if managedPath == "" {
		return
	}

	_ = os.Remove(managedPath)
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := h.Service.Logout(r.Header.Get("Authorization"))
	if err != nil {
		switch {
		case errors.Is(err, ErrMissingToken), errors.Is(err, ErrInvalidToken):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "logout failed")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handlers) Currency(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	u, err := h.Service.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		switch {
		case errors.Is(err, ErrMissingToken):
			writeError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, ErrInvalidToken):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "failed to load user currency")
		}
		return
	}

	writeJSON(w, http.StatusOK, currencyResponse{
		ShopFunds: u.ShopCurrency,
		GameFunds: u.GameCurrency,
	})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	u, err := h.Service.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		switch {
		case errors.Is(err, ErrMissingToken):
			writeError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, ErrInvalidToken):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "failed to load user")
		}
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	err = h.Service.ChangePassword(r.Context(), u.ID, ChangePasswordInput{
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrWeakPassword):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, ErrPasswordNotSet):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "password change failed")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func buildUserPublic(u *user.User) userPublic {
	if u == nil {
		return userPublic{}
	}

	username := strings.TrimSpace(u.Username)

	return userPublic{
		ID:        u.ID,
		Username:  username,
		Email:     u.Email,
		AvatarURL: u.AvatarURL,
		ShopFunds: u.ShopCurrency,
		GameFunds: u.GameCurrency,
		CreatedAt: &u.CreatedAt,
	}
}
