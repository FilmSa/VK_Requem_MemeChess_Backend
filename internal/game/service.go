package game

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"meme_chess/internal/analyzer/tree"
	"meme_chess/internal/user"
)

var (
	ErrGameNotFound       = errors.New("game not found")
	ErrForbidden          = errors.New("forbidden")
	ErrGameFull           = errors.New("game room is full")
	ErrNotYourTurn        = errors.New("not your turn")
	ErrGameFinished       = errors.New("game already finished")
	ErrGameNotActive      = errors.New("game is not active")
	ErrInvalidMove        = errors.New("invalid move")
	ErrInvalidGameMode    = errors.New("invalid game mode")
	ErrInvalidDifficulty  = errors.New("invalid bot difficulty")
	ErrInviteExpired      = errors.New("invite token expired")
	ErrInviteUsed         = errors.New("invite token already used")
	ErrInviteOwnGame      = errors.New("host cannot join own invite")
	ErrInvalidStakeRange  = errors.New("invalid stake range")
	ErrInvalidTimeControl = errors.New("invalid time control")
	ErrClockStillRunning  = errors.New("clock still running")
	ErrUntimedGame        = errors.New("untimed game")
	ErrTimeExpired        = errors.New("player time expired")
)

const defaultInviteTTL = 15 * time.Minute

type State struct {
	GameID                 string       `json:"game_id"`
	GameMode               string       `json:"game_mode"`
	BotGame                bool         `json:"bot_game,omitempty"`
	BotDifficulty          string       `json:"bot_difficulty,omitempty"`
	InitialFEN             string       `json:"initial_fen,omitempty"`
	LegalMoves             []string     `json:"legal_moves,omitempty"`
	Player1ID              string       `json:"player1_id"`
	Player2ID              string       `json:"player2_id"`
	Player1Connected       bool         `json:"player1_connected"`
	Player2Connected       bool         `json:"player2_connected"`
	Status                 string       `json:"status"`
	CurrentTurnUserID      string       `json:"current_turn_user_id"`
	BetAmount              int64        `json:"bet_amount,omitempty"`
	TimeControlID          string       `json:"time_control_id,omitempty"`
	TimeControlLabel       string       `json:"time_control_label,omitempty"`
	TimeControlBaseMs      int64        `json:"time_control_base_ms,omitempty"`
	TimeControlIncrementMs int64        `json:"time_control_increment_ms,omitempty"`
	Player1RemainingMs     int64        `json:"player1_remaining_ms,omitempty"`
	Player2RemainingMs     int64        `json:"player2_remaining_ms,omitempty"`
	CurrentTurnStartedAt   time.Time    `json:"current_turn_started_at,omitempty"`
	DrawOfferedBy          string       `json:"draw_offered_by,omitempty"`
	DrawOfferedAt          time.Time    `json:"draw_offered_at,omitempty"`
	FEN                    string       `json:"fen"`
	LastMove               string       `json:"last_move"`
	LastMoveEffects        []MoveEffect `json:"last_move_effects,omitempty"`
	WinnerID               string       `json:"winner_id,omitempty"`
	FinishedReason         string       `json:"finished_reason,omitempty"`
	RootPositionHash       string       `json:"root_position_hash"`
	CurrentPositionHash    string       `json:"current_position_hash"`
	VariantPly             int          `json:"variant_ply"`
	Moves                  []Move       `json:"moves"`
}

type GameHistoryEntry struct {
	GameID         string           `json:"game_id"`
	Status         string           `json:"status"`
	GameMode       string           `json:"game_mode"`
	BetAmount      int64            `json:"bet_amount"`
	Currency       string           `json:"currency"`
	TimeControlID  string           `json:"time_control_id,omitempty"`
	YouArePlayer1  bool             `json:"you_are_player1"`
	Opponent       *HistoryOpponent `json:"opponent,omitempty"`
	WinnerID       string           `json:"winner_id,omitempty"`
	FinishedAt     *time.Time       `json:"finished_at,omitempty"`
	FinishedReason string           `json:"finished_reason,omitempty"`
	FEN            string           `json:"fen"`
	LastMove       string           `json:"last_move"`
	LastMoveNumber int              `json:"last_move_number,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
}

type HistoryOpponent struct {
	ID        string  `json:"id"`
	Username  string  `json:"username"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type ParticipantProfile struct {
	ID        string
	Username  string
	AvatarURL *string
}

type StoredParticipants struct {
	GameID  string
	Player1 ParticipantProfile
	Player2 *ParticipantProfile
}

type Service struct {
	mu             sync.RWMutex
	sessions       map[string]*Session
	repository     *Repository
	userRepo       *user.Repository
	matchQueue     []matchRequest
	pendingMatches map[string]MatchSearchResult
	variantTracker *tree.Tracker
	moveAnalyzer   MoveAnalyzer
}

func NewService(repository *Repository) *Service {
	return &Service{
		sessions:       make(map[string]*Session),
		repository:     repository,
		matchQueue:     make([]matchRequest, 0, 32),
		pendingMatches: make(map[string]MatchSearchResult),
		variantTracker: tree.NewTracker(),
	}
}

func (s *Service) SetUserRepository(userRepo *user.Repository) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.userRepo = userRepo
}

type MatchSearchInput struct {
	UserID        string
	GameMode      string
	TimeControlID string
	MinStake      int64
	MaxStake      int64
}

type MatchSearchResult struct {
	Status       string `json:"status"`
	GameID       string `json:"game_id,omitempty"`
	AgreedStake  int64  `json:"agreed_stake,omitempty"`
	GameCurrency string `json:"game_currency,omitempty"`
	GameMode     string `json:"game_mode,omitempty"`
	timeControlPayload
}

type MatchSearchPreviewInput struct {
	UserID        string
	GameMode      string
	TimeControlID string
	MinStake      int64
	MaxStake      int64
}

type MatchSearchPreviewResult struct {
	MatchedUsersCount int64  `json:"matched_users_count"`
	GameMode          string `json:"game_mode"`
	TimeControlID     string `json:"time_control_id,omitempty"`
}

type MatchSearchLeaveResult struct {
	Status string `json:"status"`
}

type matchRequest struct {
	UserID        string
	GameMode      string
	TimeControlID string
	MinStake      int64
	MaxStake      int64
}

func (s *Service) SetMoveAnalyzer(moveAnalyzer MoveAnalyzer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.moveAnalyzer = moveAnalyzer
}

func (s *Service) CreateGame(ctx context.Context, gameID, player1ID, player2ID string, engine Engine) (*Session, error) {
	return s.CreateGameWithTimeControl(ctx, gameID, GameModeClassic, player1ID, player2ID, engine, TimeControlUnlimited)
}

func (s *Service) CreateGameWithMode(ctx context.Context, gameID, gameMode, player1ID, player2ID string, engine Engine) (*Session, error) {
	return s.CreateGameWithTimeControl(ctx, gameID, gameMode, player1ID, player2ID, engine, TimeControlUnlimited)
}

func (s *Service) CreateGameWithTimeControl(
	ctx context.Context,
	gameID,
	gameMode,
	player1ID,
	player2ID string,
	engine Engine,
	timeControlID string,
) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	mode := normalizeGameMode(gameMode)
	if mode == "" {
		return nil, ErrInvalidStakeRange
	}
	normalizedTimeControlID := normalizeTimeControlID(timeControlID)
	if normalizedTimeControlID == "" {
		return nil, ErrInvalidTimeControl
	}

	session := NewSessionWithTimeControl(gameID, mode, player1ID, player2ID, 0, engine, normalizedTimeControlID)
	s.trackSessionVariantLocked(session)
	s.sessions[gameID] = session

	if s.repository != nil {
		p2 := player2ID
		err := s.repository.CreateGame(ctx, CreateGameParams{
			GameID:                 gameID,
			Player1ID:              player1ID,
			Player2ID:              &p2,
			Status:                 string(session.Status),
			BetAmount:              0,
			MemeMode:               false,
			GameMode:               mode,
			TimeControlID:          nullableString(normalizedTimeControlID, TimeControlUnlimited),
			TimeControlBaseMs:      nullableInt64(session.TimeControlBaseMs),
			TimeControlIncrementMs: nullableInt64(session.TimeControlIncrement),
			Player1RemainingMs:     nullableInt64(session.Player1RemainingMs),
			Player2RemainingMs:     nullableInt64(session.Player2RemainingMs),
			CurrentTurnStartedAt:   nil,
			FEN:                    session.FEN,
			CurrentTurnUserID:      session.CurrentTurnUserID,
		})
		if err != nil {
			s.variantTracker.ForgetGame(gameID)
			delete(s.sessions, gameID)
			return nil, err
		}
	}

	if s.moveAnalyzer != nil {
		s.moveAnalyzer.StartGame(gameID)
	}

	return session, nil
}

func (s *Service) CreateInviteGame(ctx context.Context, hostUserID string, engine Engine) (gameID string, err error) {
	return s.CreateInviteGameWithTimeControl(ctx, GameModeClassic, hostUserID, engine, TimeControlUnlimited)
}

func (s *Service) CreateInviteGameWithMode(ctx context.Context, gameMode, hostUserID string, engine Engine) (gameID string, err error) {
	return s.CreateInviteGameWithTimeControl(ctx, gameMode, hostUserID, engine, TimeControlUnlimited)
}

func (s *Service) CreateInviteGameWithTimeControl(
	ctx context.Context,
	gameMode, hostUserID string,
	engine Engine,
	timeControlID string,
) (gameID string, err error) {
	id, err := newGameID()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[id]; exists {
		return "", errors.New("game id collision")
	}

	mode := normalizeGameMode(gameMode)
	if mode == "" {
		return "", ErrInvalidStakeRange
	}
	normalizedTimeControlID := normalizeTimeControlID(timeControlID)
	if normalizedTimeControlID == "" {
		return "", ErrInvalidTimeControl
	}

	session := NewSessionWithTimeControl(id, mode, hostUserID, "", 0, engine, normalizedTimeControlID)
	session.InviteExpiresAt = time.Now().Add(defaultInviteTTL)
	s.trackSessionVariantLocked(session)
	s.sessions[id] = session

	if s.repository != nil {
		err := s.repository.CreateGame(ctx, CreateGameParams{
			GameID:                 id,
			Player1ID:              hostUserID,
			Player2ID:              nil,
			Status:                 string(session.Status),
			BetAmount:              0,
			MemeMode:               false,
			GameMode:               mode,
			TimeControlID:          nullableString(normalizedTimeControlID, TimeControlUnlimited),
			TimeControlBaseMs:      nullableInt64(session.TimeControlBaseMs),
			TimeControlIncrementMs: nullableInt64(session.TimeControlIncrement),
			Player1RemainingMs:     nullableInt64(session.Player1RemainingMs),
			Player2RemainingMs:     nullableInt64(session.Player2RemainingMs),
			CurrentTurnStartedAt:   nil,
			FEN:                    session.FEN,
			CurrentTurnUserID:      session.CurrentTurnUserID,
		})
		if err != nil {
			s.variantTracker.ForgetGame(id)
			delete(s.sessions, id)
			return "", err
		}
	}

	if s.moveAnalyzer != nil {
		s.moveAnalyzer.StartGame(id)
	}

	return id, nil
}

func (s *Service) CreateBotGame(ctx context.Context, playerID, gameMode, difficulty string) (string, error) {
	if strings.TrimSpace(playerID) == "" {
		return "", ErrForbidden
	}

	mode := normalizeGameMode(gameMode)
	if mode == "" {
		return "", ErrInvalidGameMode
	}

	normalizedDifficulty, _, ok := normalizeBotDifficulty(difficulty)
	if !ok {
		return "", ErrInvalidDifficulty
	}

	engine, err := NewChessEngineForMode(mode)
	if err != nil {
		return "", ErrInvalidGameMode
	}

	id, err := newGameID()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[id]; exists {
		return "", errors.New("game id collision")
	}

	session := NewBotSession(id, mode, playerID, normalizedDifficulty, engine)
	s.trackSessionVariantLocked(session)
	s.sessions[id] = session

	if s.moveAnalyzer != nil {
		s.moveAnalyzer.StartGame(id)
	}

	return id, nil
}

func (s *Service) SearchMatch(ctx context.Context, in MatchSearchInput, engine Engine) (MatchSearchResult, error) {
	mode := normalizeGameMode(in.GameMode)
	timeControlID := normalizeTimeControlID(in.TimeControlID)
	if in.UserID == "" || in.MinStake <= 0 || in.MaxStake < in.MinStake || mode == "" {
		return MatchSearchResult{}, ErrInvalidStakeRange
	}
	if timeControlID == "" || timeControlID == TimeControlUnlimited {
		return MatchSearchResult{}, ErrInvalidTimeControl
	}

	s.mu.Lock()
	if pendingResult, ok := s.pendingMatches[in.UserID]; ok {
		s.mu.Unlock()
		return pendingResult, nil
	}

	for i := range s.matchQueue {
		waiting := s.matchQueue[i]
		if waiting.UserID == in.UserID {
			s.matchQueue[i] = matchRequest{
				UserID:        in.UserID,
				GameMode:      mode,
				TimeControlID: timeControlID,
				MinStake:      in.MinStake,
				MaxStake:      in.MaxStake,
			}
			s.mu.Unlock()
			return MatchSearchResult{
				Status:             "queued",
				GameMode:           mode,
				timeControlPayload: buildTimeControlPayloadFromPreset(timeControlID),
			}, nil
		}
	}

	matchIndex := -1
	for i := range s.matchQueue {
		waiting := s.matchQueue[i]
		if waiting.UserID == in.UserID {
			continue
		}
		if waiting.GameMode != mode {
			continue
		}
		if waiting.TimeControlID != timeControlID {
			continue
		}
		if !rangesOverlap(waiting.MinStake, waiting.MaxStake, in.MinStake, in.MaxStake) {
			continue
		}
		matchIndex = i
		break
	}

	if matchIndex < 0 {
		s.matchQueue = append(s.matchQueue, matchRequest{
			UserID:        in.UserID,
			GameMode:      mode,
			TimeControlID: timeControlID,
			MinStake:      in.MinStake,
			MaxStake:      in.MaxStake,
		})
		s.mu.Unlock()
		return MatchSearchResult{
			Status:             "queued",
			GameMode:           mode,
			timeControlPayload: buildTimeControlPayloadFromPreset(timeControlID),
		}, nil
	}

	waiting := s.matchQueue[matchIndex]
	s.matchQueue = append(s.matchQueue[:matchIndex], s.matchQueue[matchIndex+1:]...)
	s.mu.Unlock()

	agreedStake := maxInt64(in.MinStake, waiting.MinStake)
	matchedMode := waiting.GameMode
	matchedTimeControlID := waiting.TimeControlID
	if s.userRepo != nil {
		if err := s.userRepo.ReserveGameCurrency(ctx, in.UserID, agreedStake); err != nil {
			return MatchSearchResult{}, err
		}
		if err := s.userRepo.ReserveGameCurrency(ctx, waiting.UserID, agreedStake); err != nil {
			_ = s.userRepo.AddGameCurrency(ctx, in.UserID, agreedStake)
			return MatchSearchResult{}, err
		}
	}

	gameID, err := s.createMatchedGame(
		ctx,
		in.UserID,
		waiting.UserID,
		agreedStake,
		matchedMode,
		engine,
		matchedTimeControlID,
	)
	if err != nil {
		if s.userRepo != nil {
			_ = s.userRepo.AddGameCurrency(ctx, in.UserID, agreedStake)
			_ = s.userRepo.AddGameCurrency(ctx, waiting.UserID, agreedStake)
		}
		return MatchSearchResult{}, err
	}

	result := MatchSearchResult{
		Status:       "matched",
		GameID:       gameID,
		AgreedStake:  agreedStake,
		GameCurrency: "game_currency",
		GameMode:     matchedMode,
		timeControlPayload: buildTimeControlPayloadFromPreset(
			matchedTimeControlID,
		),
	}

	s.mu.Lock()
	s.pendingMatches[waiting.UserID] = result
	s.mu.Unlock()

	return result, nil
}

func (s *Service) PreviewMatchSearch(in MatchSearchPreviewInput) (MatchSearchPreviewResult, error) {
	mode := normalizeGameMode(in.GameMode)
	timeControlID := normalizeTimeControlID(in.TimeControlID)
	if in.MinStake <= 0 || in.MaxStake < in.MinStake || mode == "" {
		return MatchSearchPreviewResult{}, ErrInvalidStakeRange
	}
	if timeControlID == "" || timeControlID == TimeControlUnlimited {
		return MatchSearchPreviewResult{}, ErrInvalidTimeControl
	}

	userID := strings.TrimSpace(in.UserID)
	var count int64

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.matchQueue {
		waiting := s.matchQueue[i]
		if userID != "" && waiting.UserID == userID {
			continue
		}
		if waiting.GameMode != mode {
			continue
		}
		if waiting.TimeControlID != timeControlID {
			continue
		}
		if !rangesOverlap(waiting.MinStake, waiting.MaxStake, in.MinStake, in.MaxStake) {
			continue
		}
		count++
	}

	return MatchSearchPreviewResult{
		MatchedUsersCount: count,
		GameMode:          mode,
		TimeControlID:     timeControlID,
	}, nil
}

func (s *Service) LeaveMatchSearch(userID string) MatchSearchLeaveResult {
	if strings.TrimSpace(userID) == "" {
		return MatchSearchLeaveResult{Status: "idle"}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pendingMatches, userID)

	for i := range s.matchQueue {
		if s.matchQueue[i].UserID != userID {
			continue
		}
		s.matchQueue = append(s.matchQueue[:i], s.matchQueue[i+1:]...)
		return MatchSearchLeaveResult{Status: "cancelled"}
	}

	return MatchSearchLeaveResult{Status: "idle"}
}

func (s *Service) createMatchedGame(ctx context.Context, player1ID, player2ID string, stake int64, mode string, engine Engine, timeControlID string) (string, error) {
	id, err := newGameID()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[id]; exists {
		return "", errors.New("game id collision")
	}

	session := NewSessionWithTimeControl(id, mode, player1ID, player2ID, stake, engine, timeControlID)
	s.trackSessionVariantLocked(session)
	s.sessions[id] = session

	if s.repository != nil {
		p2 := player2ID
		err := s.repository.CreateGame(ctx, CreateGameParams{
			GameID:                 id,
			Player1ID:              player1ID,
			Player2ID:              &p2,
			Status:                 string(session.Status),
			BetAmount:              stake,
			MemeMode:               mode == "meme",
			GameMode:               mode,
			TimeControlID:          nullableString(timeControlID, TimeControlUnlimited),
			TimeControlBaseMs:      nullableInt64(session.TimeControlBaseMs),
			TimeControlIncrementMs: nullableInt64(session.TimeControlIncrement),
			Player1RemainingMs:     nullableInt64(session.Player1RemainingMs),
			Player2RemainingMs:     nullableInt64(session.Player2RemainingMs),
			CurrentTurnStartedAt:   nil,
			FEN:                    session.FEN,
			CurrentTurnUserID:      session.CurrentTurnUserID,
		})
		if err != nil {
			s.variantTracker.ForgetGame(id)
			delete(s.sessions, id)
			return "", err
		}
	}

	if s.moveAnalyzer != nil {
		s.moveAnalyzer.StartGame(id)
	}

	return id, nil
}

func rangesOverlap(minA, maxA, minB, maxB int64) bool {
	return minA <= maxB && minB <= maxA
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func newGameID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32], nil
}

func (s *Service) GetSession(gameID string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[gameID]
	return session, ok
}

func historyOpponentFromListRow(row UserGameListRow, viewerID string) *HistoryOpponent {
	if row.Player1ID == viewerID {
		if row.Player2ID == nil || strings.TrimSpace(*row.Player2ID) == "" {
			return nil
		}
		u := strings.TrimSpace(derefStringPtr(row.Player2Username))
		id := strings.TrimSpace(*row.Player2ID)
		return &HistoryOpponent{ID: id, Username: u, AvatarURL: row.Player2AvatarURL}
	}

	return &HistoryOpponent{
		ID:        row.Player1ID,
		Username:  strings.TrimSpace(row.Player1Username),
		AvatarURL: row.Player1AvatarURL,
	}
}

func derefStringPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *Service) GetStoredParticipants(ctx context.Context, gameID, viewerID string) (*StoredParticipants, error) {
	if strings.TrimSpace(gameID) == "" {
		return nil, ErrGameNotFound
	}
	if strings.TrimSpace(viewerID) == "" {
		return nil, ErrForbidden
	}
	if s.repository == nil {
		return nil, ErrGameNotFound
	}

	row, err := s.repository.GetGameParticipants(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, ErrGameNotFound
	}

	if viewerID != row.Player1ID && (row.Player2ID == nil || viewerID != strings.TrimSpace(*row.Player2ID)) {
		return nil, ErrForbidden
	}

	result := &StoredParticipants{
		GameID: row.GameID,
		Player1: ParticipantProfile{
			ID:        row.Player1ID,
			Username:  strings.TrimSpace(row.Player1Username),
			AvatarURL: row.Player1AvatarURL,
		},
	}

	if row.Player2ID != nil && strings.TrimSpace(*row.Player2ID) != "" {
		result.Player2 = &ParticipantProfile{
			ID:        strings.TrimSpace(*row.Player2ID),
			Username:  strings.TrimSpace(derefStringPtr(row.Player2Username)),
			AvatarURL: row.Player2AvatarURL,
		}
	}

	return result, nil
}

func (s *Service) ListUserGameHistory(ctx context.Context, userID string, limit, offset int) ([]GameHistoryEntry, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrForbidden
	}
	if s.repository == nil {
		return []GameHistoryEntry{}, nil
	}

	rows, err := s.repository.ListUserGames(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	out := make([]GameHistoryEntry, 0, len(rows))
	for _, row := range rows {
		e := GameHistoryEntry{
			GameID:        row.GameID,
			Status:        row.Status,
			GameMode:      row.GameMode,
			BetAmount:     row.BetAmount,
			Currency:      row.Currency,
			YouArePlayer1: row.Player1ID == userID,
			CreatedAt:     row.CreatedAt,
			Opponent:      historyOpponentFromListRow(row, userID),
		}
		if row.TimeControlID != nil {
			e.TimeControlID = strings.TrimSpace(*row.TimeControlID)
		}
		if row.WinnerID != nil {
			e.WinnerID = strings.TrimSpace(*row.WinnerID)
		}
		e.FinishedAt = row.FinishedAt
		if row.FinishedReason != nil {
			e.FinishedReason = strings.TrimSpace(*row.FinishedReason)
		}
		e.FEN = row.FEN
		if row.LastMoveSAN != nil {
			e.LastMove = strings.TrimSpace(*row.LastMoveSAN)
		}
		if row.LastMoveNumber != nil {
			e.LastMoveNumber = int(*row.LastMoveNumber)
		}

		if sess, ok := s.GetSession(row.GameID); ok {
			snap := sess.Snapshot()
			e.Status = snap.Status
			e.FEN = snap.FEN
			if len(snap.Moves) > 0 {
				last := snap.Moves[len(snap.Moves)-1]
				e.LastMoveNumber = last.Number
				e.LastMove = last.Move
			} else {
				e.LastMoveNumber = 0
				e.LastMove = ""
			}
			if snap.BotGame {
				e.Opponent = &HistoryOpponent{
					ID:       botUserID,
					Username: botDisplayName(),
				}
			}
			if strings.TrimSpace(snap.WinnerID) != "" {
				e.WinnerID = snap.WinnerID
			}
			if strings.TrimSpace(snap.FinishedReason) != "" {
				e.FinishedReason = snap.FinishedReason
			}
		}

		out = append(out, e)
	}
	return out, nil
}

func (s *Service) JoinGame(ctx context.Context, gameID, userID string) (State, error) {
	session, ok := s.GetSession(gameID)
	if !ok {
		return State{}, ErrGameNotFound
	}

	if session.HasPlayer(userID) {
		s.mu.Lock()
		delete(s.pendingMatches, userID)
		s.mu.Unlock()
		session.SetConnected(userID, true)
		state := session.Snapshot()
		if err := s.persistNonTerminalState(ctx, state); err != nil {
			return State{}, err
		}
		return state, nil
	}

	if session.IsInviteExpired(time.Now()) {
		return State{}, ErrInviteExpired
	}

	if err := session.AssignPlayer2(userID); err != nil {
		return State{}, err
	}

	if s.repository != nil {
		if err := s.repository.SetPlayer2(ctx, gameID, userID); err != nil {
			session.RollbackPlayer2If(userID)
			if errors.Is(err, ErrOpponentSeatTaken) {
				return State{}, ErrGameFull
			}
			return State{}, err
		}
	}

	s.mu.Lock()
	delete(s.pendingMatches, userID)
	s.mu.Unlock()
	session.SetConnected(userID, true)
	state := session.Snapshot()
	if err := s.persistNonTerminalState(ctx, state); err != nil {
		return State{}, err
	}
	return state, nil
}

func (s *Service) ReserveInviteSeat(ctx context.Context, inviteToken, userID string) (State, error) {
	session, ok := s.GetSession(inviteToken)
	if !ok {
		return State{}, ErrGameNotFound
	}

	if session.Snapshot().Player1ID == userID {
		return State{}, ErrInviteOwnGame
	}

	if session.HasPlayer(userID) {
		return session.Snapshot(), nil
	}

	if err := session.ReserveInviteSeat(userID, time.Now()); err != nil {
		return State{}, err
	}

	if s.repository != nil {
		if err := s.repository.SetPlayer2(ctx, inviteToken, userID); err != nil {
			session.RollbackPlayer2If(userID)
			if errors.Is(err, ErrOpponentSeatTaken) {
				return State{}, ErrInviteUsed
			}
			return State{}, err
		}
	}

	return session.Snapshot(), nil
}

func (s *Service) LeaveGame(gameID, userID string) error {
	session, ok := s.GetSession(gameID)
	if !ok {
		return ErrGameNotFound
	}
	if !session.HasPlayer(userID) {
		return ErrForbidden
	}

	session.SetConnected(userID, false)
	return nil
}

func (s *Service) MakeMove(ctx context.Context, gameID, userID, move string) (State, MoveResult, error) {
	session, ok := s.GetSession(gameID)
	if !ok {
		return State{}, MoveResult{}, ErrGameNotFound
	}
	if !session.HasPlayer(userID) {
		return State{}, MoveResult{}, ErrForbidden
	}
	if move == "" {
		return State{}, MoveResult{}, ErrInvalidMove
	}

	state, result, err := session.ApplyMove(userID, move)
	if err != nil {
		if errors.Is(err, ErrTimeExpired) {
			if persistErr := s.persistFinishedState(ctx, state); persistErr != nil {
				return State{}, MoveResult{}, persistErr
			}
			if payoutErr := s.settlePayoutIfNeeded(ctx, session, state); payoutErr != nil {
				return State{}, MoveResult{}, payoutErr
			}
		}
		return State{}, MoveResult{}, err
	}
	result = s.decorateMoveWithMeme(gameID, session, state, result)

	cursor, err := s.variantTracker.AdvanceGame(gameID, result.Move, state.FEN)
	if err != nil {
		return State{}, MoveResult{}, err
	}
	session.SetVariantCursor(cursor.RootPositionHash, cursor.CurrentPositionHash, cursor.Ply)
	state = session.Snapshot()

	if s.repository != nil && !session.IsBotGame() {
		moveNumber := len(state.Moves)

		if err := s.repository.SaveMove(ctx, SaveMoveParams{
			GameID:       gameID,
			PlayerID:     userID,
			MoveNumber:   moveNumber,
			Move:         result.Move,
			FEN:          result.FEN,
			IsCapture:    result.IsCapture,
			IsCheck:      result.IsCheck,
			IsCheckmate:  result.IsCheckmate,
			MemeID:       nullableString(result.MemeID, ""),
			MemeCategory: nullableString(result.MemeCategory, ""),
		}); err != nil {
			return State{}, MoveResult{}, err
		}

		var winnerID *string
		var finishedAt *time.Time
		var finishedReason *string

		if state.WinnerID != "" {
			winnerID = &state.WinnerID
			now := time.Now()
			finishedAt = &now
		}
		if strings.TrimSpace(state.FinishedReason) != "" {
			r := state.FinishedReason
			finishedReason = &r
		}

		if err := s.repository.UpdateGameState(ctx, UpdateGameStateParams{
			GameID:            gameID,
			Status:            state.Status,
			FEN:               state.FEN,
			CurrentTurnUserID: state.CurrentTurnUserID,
			WinnerID:          winnerID,
			FinishedAt:        finishedAt,
			FinishedReason:    finishedReason,
		}); err != nil {
			return State{}, MoveResult{}, err
		}
	}

	if err := s.settlePayoutIfNeeded(ctx, session, state); err != nil {
		return State{}, MoveResult{}, err
	}

	if s.moveAnalyzer != nil {
		s.moveAnalyzer.RecordMove(gameID, result.Move)
	}

	return state, result, nil
}

func (s *Service) PlayBotTurn(ctx context.Context, gameID string) (State, MoveResult, bool, error) {
	session, ok := s.GetSession(gameID)
	if !ok {
		return State{}, MoveResult{}, false, ErrGameNotFound
	}
	if !session.IsBotGame() {
		return session.Snapshot(), MoveResult{}, false, nil
	}

	snapshot := session.Snapshot()
	if snapshot.Status != string(StatusActive) || snapshot.CurrentTurnUserID != botUserID {
		return snapshot, MoveResult{}, false, nil
	}

	move, err := chooseBotMove(session.engine, snapshot.BotDifficulty)
	if err != nil {
		if errors.Is(err, errNoLegalBotMove) {
			return session.FinishDraw("stalemate"), MoveResult{}, false, nil
		}
		return State{}, MoveResult{}, false, err
	}

	state, result, err := session.ApplyMove(botUserID, move)
	if err != nil {
		return State{}, MoveResult{}, false, err
	}
	result = s.decorateMoveWithMeme(gameID, session, state, result)

	cursor, err := s.variantTracker.AdvanceGame(gameID, result.Move, state.FEN)
	if err != nil {
		return State{}, MoveResult{}, false, err
	}
	session.SetVariantCursor(cursor.RootPositionHash, cursor.CurrentPositionHash, cursor.Ply)
	state = session.Snapshot()

	if s.moveAnalyzer != nil {
		s.moveAnalyzer.RecordMove(gameID, result.Move)
	}

	return state, result, true, nil
}

func (s *Service) trackSessionVariantLocked(session *Session) {
	cursor := s.variantTracker.TrackGame(session.GameID, session.FEN)
	session.SetVariantCursor(cursor.RootPositionHash, cursor.CurrentPositionHash, cursor.Ply)
}

func (s *Service) Resign(ctx context.Context, gameID, userID string) (State, error) {
	session, ok := s.GetSession(gameID)
	if !ok {
		return State{}, ErrGameNotFound
	}
	if !session.HasPlayer(userID) {
		return State{}, ErrForbidden
	}

	state, err := session.Resign(userID)
	if err != nil {
		return State{}, err
	}

	if err := s.persistFinishedState(ctx, state); err != nil {
		return State{}, err
	}
	if err := s.settlePayoutIfNeeded(ctx, session, state); err != nil {
		return State{}, err
	}
	return state, nil
}

func (s *Service) OfferDraw(ctx context.Context, gameID, userID string) (State, error) {
	session, ok := s.GetSession(gameID)
	if !ok {
		return State{}, ErrGameNotFound
	}
	if !session.HasPlayer(userID) {
		return State{}, ErrForbidden
	}

	state, err := session.OfferDraw(userID, time.Now())
	if err != nil {
		return State{}, err
	}

	if err := s.persistNonTerminalState(ctx, state); err != nil {
		return State{}, err
	}
	return state, nil
}

func (s *Service) DeclineDraw(ctx context.Context, gameID, userID string) (State, error) {
	session, ok := s.GetSession(gameID)
	if !ok {
		return State{}, ErrGameNotFound
	}
	if !session.HasPlayer(userID) {
		return State{}, ErrForbidden
	}

	state, err := session.DeclineDraw(userID)
	if err != nil {
		return State{}, err
	}

	if err := s.persistNonTerminalState(ctx, state); err != nil {
		return State{}, err
	}
	return state, nil
}

func (s *Service) AcceptDraw(ctx context.Context, gameID, userID string) (State, error) {
	session, ok := s.GetSession(gameID)
	if !ok {
		return State{}, ErrGameNotFound
	}
	if !session.HasPlayer(userID) {
		return State{}, ErrForbidden
	}

	state, err := session.AcceptDraw(userID)
	if err != nil {
		return State{}, err
	}

	if err := s.persistFinishedState(ctx, state); err != nil {
		return State{}, err
	}
	if err := s.settlePayoutIfNeeded(ctx, session, state); err != nil {
		return State{}, err
	}
	return state, nil
}

func (s *Service) Timeout(ctx context.Context, gameID, requesterID string) (State, error) {
	session, ok := s.GetSession(gameID)
	if !ok {
		return State{}, ErrGameNotFound
	}
	if !session.HasPlayer(requesterID) {
		return State{}, ErrForbidden
	}

	state, err := session.Timeout(time.Now())
	if err != nil {
		return State{}, err
	}
	if err := s.persistFinishedState(ctx, state); err != nil {
		return State{}, err
	}
	if err := s.settlePayoutIfNeeded(ctx, session, state); err != nil {
		return State{}, err
	}
	return state, nil
}

func (s *Service) persistFinishedState(ctx context.Context, state State) error {
	if isBotUserID(state.Player2ID) {
		return nil
	}
	if s.repository == nil {
		return nil
	}

	var winnerID *string
	if strings.TrimSpace(state.WinnerID) != "" {
		w := state.WinnerID
		winnerID = &w
	}

	now := time.Now()
	finishedAt := &now

	var finishedReason *string
	if strings.TrimSpace(state.FinishedReason) != "" {
		r := state.FinishedReason
		finishedReason = &r
	}

	return s.repository.UpdateGameState(ctx, UpdateGameStateParams{
		GameID:                 state.GameID,
		Status:                 state.Status,
		FEN:                    state.FEN,
		CurrentTurnUserID:      state.CurrentTurnUserID,
		TimeControlID:          nullableString(state.TimeControlID, TimeControlUnlimited),
		TimeControlBaseMs:      nullableInt64(state.TimeControlBaseMs),
		TimeControlIncrementMs: nullableInt64(state.TimeControlIncrementMs),
		Player1RemainingMs:     nullableInt64(state.Player1RemainingMs),
		Player2RemainingMs:     nullableInt64(state.Player2RemainingMs),
		CurrentTurnStartedAt:   nil,
		WinnerID:               winnerID,
		FinishedAt:             finishedAt,
		FinishedReason:         finishedReason,
	})
}

func (s *Service) persistNonTerminalState(ctx context.Context, state State) error {
	if isBotUserID(state.Player2ID) {
		return nil
	}
	if s.repository == nil {
		return nil
	}

	return s.repository.UpdateGameState(ctx, UpdateGameStateParams{
		GameID:                 state.GameID,
		Status:                 state.Status,
		FEN:                    state.FEN,
		CurrentTurnUserID:      state.CurrentTurnUserID,
		TimeControlID:          nullableString(state.TimeControlID, TimeControlUnlimited),
		TimeControlBaseMs:      nullableInt64(state.TimeControlBaseMs),
		TimeControlIncrementMs: nullableInt64(state.TimeControlIncrementMs),
		Player1RemainingMs:     nullableInt64(state.Player1RemainingMs),
		Player2RemainingMs:     nullableInt64(state.Player2RemainingMs),
		CurrentTurnStartedAt:   nullableTime(state.CurrentTurnStartedAt),
		WinnerID:               nil,
		FinishedAt:             nil,
		FinishedReason:         nil,
	})
}

func (s *Service) settlePayoutIfNeeded(ctx context.Context, session *Session, state State) error {
	if s.repository == nil || s.userRepo == nil {
		return nil
	}
	if state.Status != string(StatusFinished) {
		return nil
	}
	if session == nil || session.BetAmount <= 0 {
		return nil
	}

	ok, err := s.repository.TryMarkPaidOut(ctx, state.GameID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	bet := session.BetAmount
	switch strings.TrimSpace(state.WinnerID) {
	case "":
		if err := s.userRepo.AddGameCurrency(ctx, state.Player1ID, bet); err != nil {
			return err
		}
		if err := s.userRepo.AddGameCurrency(ctx, state.Player2ID, bet); err != nil {
			return err
		}
	default:
		if err := s.userRepo.AddGameCurrency(ctx, state.WinnerID, bet*2); err != nil {
			return err
		}
	}

	return nil
}

func nullableString(value string, emptySentinel string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == emptySentinel {
		return nil
	}
	return &trimmed
}

func nullableInt64(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func nullableTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}
