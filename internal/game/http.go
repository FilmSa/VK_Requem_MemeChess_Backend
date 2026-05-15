package game

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"meme_chess/internal/auth"
	"meme_chess/internal/user"
)

type HTTP struct {
	Svc               *Service
	JWT               *auth.JWTManager
	AuthService       *auth.Service
	UserRepo          *user.Repository
	JoinBase          string // e.g. http://localhost:5173
	BroadcastState    func(string, State)
	BroadcastFinished func(string, State)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAuthError(w http.ResponseWriter, err error) {
	msg := "unauthorized"
	if err != nil {
		msg = err.Error()
	}
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": msg})
}

type inviteCreateResponse struct {
	GameID   string `json:"game_id"`
	GameMode string `json:"game_mode"`
	timeControlPayload
	InviteToken string    `json:"invite_token"`
	InviteURL   string    `json:"invite_url"`
	JoinURL     string    `json:"join_url"`
	ExpiresAt   time.Time `json:"expires_at"`
	Status      string    `json:"status"`
}

type inviteCreateRequest struct {
	GameMode      string `json:"game_mode"`
	TimeControlID string `json:"time_control_id"`
}

type inviteParticipant struct {
	ID        string  `json:"id"`
	Username  string  `json:"username"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	IsGuest   bool    `json:"is_guest"`
}

type inviteJoinResponse struct {
	GameID string `json:"game_id"`
	timeControlPayload
	InviteToken  string            `json:"invite_token"`
	PlayURL      string            `json:"play_url"`
	GameMode     string            `json:"game_mode"`
	SessionToken string            `json:"session_token"`
	Player       inviteParticipant `json:"player"`
	Status       string            `json:"status"`
}

type participantsResponse struct {
	GameID  string             `json:"game_id"`
	Player1 *inviteParticipant `json:"player1,omitempty"`
	Player2 *inviteParticipant `json:"player2,omitempty"`
}

type moveAnalysisRequest struct {
	Move       string `json:"move"`
	MoveNumber int    `json:"move_number,omitempty"`
	Depth      int    `json:"depth,omitempty"`
}

type matchSearchRequest struct {
	GameMode      string `json:"game_mode"`
	TimeControlID string `json:"time_control_id"`
	MinStake      int64  `json:"min_stake"`
	MaxStake      int64  `json:"max_stake"`
}

type robotCreateRequest struct {
	GameMode   string `json:"game_mode"`
	Difficulty string `json:"difficulty"`
}

type robotCreateResponse struct {
	GameID        string            `json:"game_id"`
	GameMode      string            `json:"game_mode"`
	BotDifficulty string            `json:"bot_difficulty"`
	PlayURL       string            `json:"play_url"`
	Status        string            `json:"status"`
	Opponent      inviteParticipant `json:"opponent"`
}

func (h *HTTP) PostResign(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	participant, err := h.AuthService.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeAuthError(w, err)
		return
	}

	state, err := h.Svc.Resign(r.Context(), gameID, participant.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrGameNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		case errors.Is(err, ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a participant"})
		case errors.Is(err, ErrGameFinished):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "game already finished"})
		case errors.Is(err, ErrGameNotActive):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "game is not active"})
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return
	}

	writeJSON(w, http.StatusOK, state)
}

func (h *HTTP) PostDrawOffer(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	participant, err := h.AuthService.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeAuthError(w, err)
		return
	}

	state, err := h.Svc.OfferDraw(r.Context(), gameID, participant.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrGameNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		case errors.Is(err, ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a participant"})
		case errors.Is(err, ErrGameFinished):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "game already finished"})
		case errors.Is(err, ErrGameNotActive):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "game is not active"})
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return
	}

	writeJSON(w, http.StatusOK, state)
}

func (h *HTTP) PostDrawAccept(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	participant, err := h.AuthService.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeAuthError(w, err)
		return
	}

	state, err := h.Svc.AcceptDraw(r.Context(), gameID, participant.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrGameNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		case errors.Is(err, ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "draw offer required"})
		case errors.Is(err, ErrGameFinished):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "game already finished"})
		case errors.Is(err, ErrGameNotActive):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "game is not active"})
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return
	}

	writeJSON(w, http.StatusOK, state)
}

func (h *HTTP) PostDrawDecline(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	participant, err := h.AuthService.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeAuthError(w, err)
		return
	}

	state, err := h.Svc.DeclineDraw(r.Context(), gameID, participant.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrGameNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		case errors.Is(err, ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		case errors.Is(err, ErrGameFinished):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "game already finished"})
		case errors.Is(err, ErrGameNotActive):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "game is not active"})
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return
	}

	writeJSON(w, http.StatusOK, state)
}

// PostInvite creates a new room; host shares JoinBase/invite/{invite_token} with the opponent.
func (h *HTTP) PostInvite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req inviteCreateRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	claims, err := h.JWT.ClaimsFromAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		writeAuthError(w, err)
		return
	}

	mode := normalizeGameMode(req.GameMode)
	if mode == "" && strings.TrimSpace(req.GameMode) != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid game_mode"})
		return
	}
	if mode == "" {
		mode = GameModeClassic
	}

	engine, err := NewChessEngineForMode(mode)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid game_mode"})
		return
	}

	timeControlID := normalizeTimeControlID(req.TimeControlID)
	if timeControlID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid time_control_id"})
		return
	}

	gameID, err := h.Svc.CreateInviteGameWithTimeControl(r.Context(), mode, claims.UserID, engine, timeControlID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidTimeControl):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid time_control_id"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create game"})
		}
		return
	}

	session, _ := h.Svc.GetSession(gameID)
	expiresAt := time.Now().Add(defaultInviteTTL)
	if session != nil {
		expiresAt = session.InviteDeadline()
	}

	inviteURL := h.JoinBase + "/invite/" + gameID
	writeJSON(w, http.StatusCreated, inviteCreateResponse{
		GameID:             gameID,
		GameMode:           mode,
		timeControlPayload: buildTimeControlPayloadFromPreset(timeControlID),
		InviteToken:        gameID,
		InviteURL:          inviteURL,
		JoinURL:            inviteURL,
		ExpiresAt:          expiresAt,
		Status:             string(StatusWaiting),
	})
}

func (h *HTTP) PostRobotGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	participant, err := h.AuthService.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeAuthError(w, err)
		return
	}

	var req robotCreateRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	mode := normalizeGameMode(req.GameMode)
	if mode == "" && strings.TrimSpace(req.GameMode) != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid game_mode"})
		return
	}
	if mode == "" {
		mode = GameModeClassic
	}

	difficulty, _, ok := normalizeBotDifficulty(req.Difficulty)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid difficulty"})
		return
	}

	gameID, err := h.Svc.CreateBotGame(r.Context(), participant.ID, mode, difficulty)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidGameMode):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid game_mode"})
		case errors.Is(err, ErrInvalidDifficulty):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid difficulty"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create robot game"})
		}
		return
	}

	writeJSON(w, http.StatusCreated, robotCreateResponse{
		GameID:        gameID,
		GameMode:      mode,
		BotDifficulty: difficulty,
		PlayURL:       h.JoinBase + "/play?game=" + gameID,
		Status:        string(StatusActive),
		Opponent:      buildBotParticipant(),
	})
}

func (h *HTTP) PostInviteJoin(w http.ResponseWriter, r *http.Request, inviteToken string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	inviteToken = strings.TrimSpace(inviteToken)
	if inviteToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invite token is required"})
		return
	}

	sessionToken, participant, err := h.resolveInviteParticipant(r)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	state, err := h.Svc.ReserveInviteSeat(r.Context(), inviteToken, participant.ID)
	if err != nil {
		h.writeInviteJoinError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, inviteJoinResponse{
		GameID: state.GameID,
		timeControlPayload: buildTimeControlPayload(
			state.TimeControlID,
			state.TimeControlLabel,
			state.TimeControlBaseMs,
			state.TimeControlIncrementMs,
			state.Player1RemainingMs,
			state.Player2RemainingMs,
			state.CurrentTurnStartedAt,
		),
		InviteToken:  inviteToken,
		PlayURL:      h.JoinBase + "/play?game=" + state.GameID,
		GameMode:     state.GameMode,
		SessionToken: sessionToken,
		Player:       buildInviteParticipant(participant),
		Status:       state.Status,
	})
}

// guestGamePreview is returned to logged-in users who may take the second seat.
type guestGamePreview struct {
	GameID   string `json:"game_id"`
	Status   string `json:"status"`
	OpenSeat bool   `json:"open_seat"`
}

// GetGame returns full state for participants, or a short preview if the second seat is open.
func (h *HTTP) GetGame(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, err := h.JWT.ClaimsFromAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		writeAuthError(w, err)
		return
	}

	session, ok := h.Svc.GetSession(gameID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		return
	}

	snap := session.Snapshot()
	if claims.UserID == snap.Player1ID || (snap.Player2ID != "" && claims.UserID == snap.Player2ID) {
		writeJSON(w, http.StatusOK, snap)
		return
	}

	if snap.Player2ID == "" && claims.UserID != snap.Player1ID {
		writeJSON(w, http.StatusOK, guestGamePreview{
			GameID:   snap.GameID,
			Status:   snap.Status,
			OpenSeat: true,
		})
		return
	}

	writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a participant"})
}

func (h *HTTP) GetParticipants(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	participant, err := h.AuthService.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeAuthError(w, err)
		return
	}

	session, ok := h.Svc.GetSession(gameID)
	if ok {
		snapshot := session.Snapshot()
		if participant.ID != snapshot.Player1ID && participant.ID != snapshot.Player2ID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a participant"})
			return
		}

		player1, err := h.loadParticipant(r, snapshot.Player1ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load player profile"})
			return
		}

		var player2 *inviteParticipant
		if snapshot.BotGame {
			bot := buildBotParticipant()
			player2 = &bot
		} else {
			player2, err = h.loadParticipant(r, snapshot.Player2ID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load player profile"})
				return
			}
		}

		writeJSON(w, http.StatusOK, participantsResponse{
			GameID:  snapshot.GameID,
			Player1: player1,
			Player2: player2,
		})
		return
	}

	storedParticipants, err := h.Svc.GetStoredParticipants(r.Context(), gameID, participant.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrGameNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		case errors.Is(err, ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a participant"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load player profile"})
		}
		return
	}

	player1 := &inviteParticipant{
		ID:        storedParticipants.Player1.ID,
		Username:  storedParticipants.Player1.Username,
		AvatarURL: storedParticipants.Player1.AvatarURL,
		IsGuest:   auth.IsGuestUsername(storedParticipants.Player1.Username),
	}

	var player2 *inviteParticipant
	if storedParticipants.Player2 != nil {
		player2 = &inviteParticipant{
			ID:        storedParticipants.Player2.ID,
			Username:  storedParticipants.Player2.Username,
			AvatarURL: storedParticipants.Player2.AvatarURL,
			IsGuest:   auth.IsGuestUsername(storedParticipants.Player2.Username),
		}
	}

	writeJSON(w, http.StatusOK, participantsResponse{
		GameID:  storedParticipants.GameID,
		Player1: player1,
		Player2: player2,
	})
}

func (h *HTTP) GetMyGameHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	participant, err := h.AuthService.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeAuthError(w, err)
		return
	}

	limit := 50
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
			if limit > 100 {
				limit = 100
			}
		}
	}
	offset := 0
	if v := strings.TrimSpace(r.URL.Query().Get("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	games, err := h.Svc.ListUserGameHistory(r.Context(), participant.ID, limit+1, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load game history"})
		return
	}

	hasMore := len(games) > limit
	if hasMore {
		games = games[:limit]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"games":       games,
		"limit":       limit,
		"offset":      offset,
		"has_more":    hasMore,
		"next_offset": offset + len(games),
	})
}

func (h *HTTP) PostAnalyzeMove(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	participant, err := h.AuthService.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeAuthError(w, err)
		return
	}

	var req moveAnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	result, err := h.Svc.AnalyzeMove(gameID, participant.ID, req.Move, req.MoveNumber, req.Depth)
	if err != nil {
		switch {
		case errors.Is(err, ErrGameNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		case errors.Is(err, ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a participant"})
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"result": result,
	})
}

func (h *HTTP) PostMatchSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	participant, err := h.AuthService.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeAuthError(w, err)
		return
	}

	var req matchSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	engine, err := NewChessEngineForMode(req.GameMode)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid game_mode or stake range"})
		return
	}

	result, err := h.Svc.SearchMatch(r.Context(), MatchSearchInput{
		UserID:        participant.ID,
		GameMode:      req.GameMode,
		TimeControlID: req.TimeControlID,
		MinStake:      req.MinStake,
		MaxStake:      req.MaxStake,
	}, engine)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidStakeRange):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid game_mode or stake range"})
		case errors.Is(err, ErrInvalidTimeControl):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid time_control_id"})
		case errors.Is(err, user.ErrInsufficientGameCurrency):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "insufficient game currency"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to search match"})
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *HTTP) PostMatchSearchPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	participant, err := h.AuthService.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeAuthError(w, err)
		return
	}

	var req matchSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	result, err := h.Svc.PreviewMatchSearch(MatchSearchPreviewInput{
		UserID:        participant.ID,
		GameMode:      req.GameMode,
		TimeControlID: req.TimeControlID,
		MinStake:      req.MinStake,
		MaxStake:      req.MaxStake,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidStakeRange):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid game_mode or stake range"})
		case errors.Is(err, ErrInvalidTimeControl):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid time_control_id"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to preview match search"})
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *HTTP) PostMatchSearchLeave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	participant, err := h.AuthService.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeAuthError(w, err)
		return
	}

	result := h.Svc.LeaveMatchSearch(participant.ID)
	writeJSON(w, http.StatusOK, result)
}

func (h *HTTP) PostTimeout(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	participant, err := h.AuthService.UserFromBearer(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		writeAuthError(w, err)
		return
	}

	state, err := h.Svc.Timeout(r.Context(), gameID, participant.ID)
	if err != nil {
		switch {
		case errors.Is(err, ErrGameNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		case errors.Is(err, ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a participant"})
		case errors.Is(err, ErrGameFinished):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "game already finished"})
		case errors.Is(err, ErrGameNotActive):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "game is not active"})
		case errors.Is(err, ErrUntimedGame):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "game has no time control"})
		case errors.Is(err, ErrClockStillRunning):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "clock still running"})
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return
	}

	if h.BroadcastState != nil {
		h.BroadcastState(gameID, state)
	}
	if h.BroadcastFinished != nil {
		h.BroadcastFinished(gameID, state)
	}
	writeJSON(w, http.StatusOK, state)
}

func (h *HTTP) resolveInviteParticipant(r *http.Request) (string, *user.User, error) {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if authorization == "" {
		if h.AuthService == nil {
			return "", nil, errors.New("guest auth is unavailable")
		}
		return h.AuthService.CreateGuestSession(r.Context())
	}

	u, err := h.AuthService.UserFromBearer(r.Context(), authorization)
	if err != nil {
		return "", nil, err
	}

	return strings.TrimSpace(authorization[7:]), u, nil
}

func (h *HTTP) writeInviteJoinError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrGameNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "invite token is invalid"})
	case errors.Is(err, ErrInviteExpired):
		writeJSON(w, http.StatusGone, map[string]string{"error": "invite token expired"})
	case errors.Is(err, ErrInviteUsed):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "invite token already used"})
	case errors.Is(err, ErrInviteOwnGame):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "host cannot join own invite"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to join invite"})
	}
}

func buildInviteParticipant(u *user.User) inviteParticipant {
	if u == nil {
		return inviteParticipant{}
	}

	return inviteParticipant{
		ID:        u.ID,
		Username:  strings.TrimSpace(u.Username),
		AvatarURL: u.AvatarURL,
		IsGuest:   auth.IsGuestUsername(u.Username),
	}
}

func buildBotParticipant() inviteParticipant {
	return inviteParticipant{
		ID:       botUserID,
		Username: botDisplayName(),
		IsGuest:  false,
	}
}

func (h *HTTP) loadParticipant(r *http.Request, userID string) (*inviteParticipant, error) {
	if strings.TrimSpace(userID) == "" || h.UserRepo == nil {
		return nil, nil
	}

	u, err := h.UserRepo.GetByID(r.Context(), userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, nil
	}

	profile := buildInviteParticipant(u)
	return &profile, nil
}
