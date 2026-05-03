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
	ErrGameNotFound      = errors.New("game not found")
	ErrForbidden         = errors.New("forbidden")
	ErrGameFull          = errors.New("game room is full")
	ErrNotYourTurn       = errors.New("not your turn")
	ErrGameFinished      = errors.New("game already finished")
	ErrGameNotActive     = errors.New("game is not active")
	ErrInvalidMove       = errors.New("invalid move")
	ErrInvalidGameMode   = errors.New("invalid game mode")
	ErrInvalidDifficulty = errors.New("invalid bot difficulty")
	ErrInviteExpired     = errors.New("invite token expired")
	ErrInviteUsed        = errors.New("invite token already used")
	ErrInviteOwnGame     = errors.New("host cannot join own invite")
	ErrInvalidStakeRange = errors.New("invalid stake range")
)

const defaultInviteTTL = 15 * time.Minute

type State struct {
	GameID              string    `json:"game_id"`
	GameMode            string    `json:"game_mode"`
	BotGame             bool      `json:"bot_game,omitempty"`
	BotDifficulty       string    `json:"bot_difficulty,omitempty"`
	InitialFEN          string    `json:"initial_fen,omitempty"`
	LegalMoves          []string  `json:"legal_moves,omitempty"`
	Player1ID           string    `json:"player1_id"`
	Player2ID           string    `json:"player2_id"`
	Player1Connected    bool      `json:"player1_connected"`
	Player2Connected    bool      `json:"player2_connected"`
	Status              string    `json:"status"`
	CurrentTurnUserID   string    `json:"current_turn_user_id"`
	BetAmount           int64     `json:"bet_amount,omitempty"`
	DrawOfferedBy       string    `json:"draw_offered_by,omitempty"`
	DrawOfferedAt       time.Time `json:"draw_offered_at,omitempty"`
	FEN                 string    `json:"fen"`
	LastMove            string    `json:"last_move"`
	WinnerID            string    `json:"winner_id,omitempty"`
	FinishedReason      string    `json:"finished_reason,omitempty"`
	RootPositionHash    string    `json:"root_position_hash"`
	CurrentPositionHash string    `json:"current_position_hash"`
	VariantPly          int       `json:"variant_ply"`
	Moves               []Move    `json:"moves"`
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
	UserID   string
	GameMode string
	MinStake int64
	MaxStake int64
}

type MatchSearchResult struct {
	Status       string `json:"status"`
	GameID       string `json:"game_id,omitempty"`
	AgreedStake  int64  `json:"agreed_stake,omitempty"`
	GameCurrency string `json:"game_currency,omitempty"`
	GameMode     string `json:"game_mode,omitempty"`
}

type MatchSearchPreviewInput struct {
	UserID   string
	GameMode string
	MinStake int64
	MaxStake int64
}

type MatchSearchPreviewResult struct {
	MatchedUsersCount int64  `json:"matched_users_count"`
	GameMode          string `json:"game_mode"`
}

type MatchSearchLeaveResult struct {
	Status string `json:"status"`
}

type matchRequest struct {
	UserID   string
	GameMode string
	MinStake int64
	MaxStake int64
}

func (s *Service) SetMoveAnalyzer(moveAnalyzer MoveAnalyzer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.moveAnalyzer = moveAnalyzer
}

func (s *Service) CreateGame(ctx context.Context, gameID, player1ID, player2ID string, engine Engine) (*Session, error) {
	return s.CreateGameWithMode(ctx, gameID, GameModeClassic, player1ID, player2ID, engine)
}

func (s *Service) CreateGameWithMode(ctx context.Context, gameID, gameMode, player1ID, player2ID string, engine Engine) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	mode := normalizeGameMode(gameMode)
	if mode == "" {
		return nil, ErrInvalidStakeRange
	}

	session := NewSession(gameID, mode, player1ID, player2ID, 0, engine)
	s.trackSessionVariantLocked(session)
	s.sessions[gameID] = session

	if s.repository != nil {
		p2 := player2ID
		err := s.repository.CreateGame(ctx, CreateGameParams{
			GameID:            gameID,
			Player1ID:         player1ID,
			Player2ID:         &p2,
			Status:            string(session.Status),
			BetAmount:         0,
			MemeMode:          false,
			GameMode:          mode,
			FEN:               session.FEN,
			CurrentTurnUserID: session.CurrentTurnUserID,
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
	return s.CreateInviteGameWithMode(ctx, GameModeClassic, hostUserID, engine)
}

func (s *Service) CreateInviteGameWithMode(ctx context.Context, gameMode, hostUserID string, engine Engine) (gameID string, err error) {
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

	session := NewSession(id, mode, hostUserID, "", 0, engine)
	session.InviteExpiresAt = time.Now().Add(defaultInviteTTL)
	s.trackSessionVariantLocked(session)
	s.sessions[id] = session

	if s.repository != nil {
		err := s.repository.CreateGame(ctx, CreateGameParams{
			GameID:            id,
			Player1ID:         hostUserID,
			Player2ID:         nil,
			Status:            string(session.Status),
			BetAmount:         0,
			MemeMode:          false,
			GameMode:          mode,
			FEN:               session.FEN,
			CurrentTurnUserID: session.CurrentTurnUserID,
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
	if in.UserID == "" || in.MinStake <= 0 || in.MaxStake < in.MinStake || mode == "" {
		return MatchSearchResult{}, ErrInvalidStakeRange
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
				UserID:   in.UserID,
				GameMode: mode,
				MinStake: in.MinStake,
				MaxStake: in.MaxStake,
			}
			s.mu.Unlock()
			return MatchSearchResult{Status: "queued", GameMode: mode}, nil
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
		if !rangesOverlap(waiting.MinStake, waiting.MaxStake, in.MinStake, in.MaxStake) {
			continue
		}
		matchIndex = i
		break
	}

	if matchIndex < 0 {
		s.matchQueue = append(s.matchQueue, matchRequest{
			UserID:   in.UserID,
			GameMode: mode,
			MinStake: in.MinStake,
			MaxStake: in.MaxStake,
		})
		s.mu.Unlock()
		return MatchSearchResult{Status: "queued", GameMode: mode}, nil
	}

	waiting := s.matchQueue[matchIndex]
	s.matchQueue = append(s.matchQueue[:matchIndex], s.matchQueue[matchIndex+1:]...)
	s.mu.Unlock()

	agreedStake := maxInt64(in.MinStake, waiting.MinStake)
	if s.userRepo != nil {
		if err := s.userRepo.ReserveGameCurrency(ctx, in.UserID, agreedStake); err != nil {
			return MatchSearchResult{}, err
		}
		if err := s.userRepo.ReserveGameCurrency(ctx, waiting.UserID, agreedStake); err != nil {
			_ = s.userRepo.AddGameCurrency(ctx, in.UserID, agreedStake)
			return MatchSearchResult{}, err
		}
	}

	gameID, err := s.createMatchedGame(ctx, in.UserID, waiting.UserID, agreedStake, mode, engine)
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
		GameMode:     mode,
	}

	s.mu.Lock()
	s.pendingMatches[waiting.UserID] = result
	s.mu.Unlock()

	return result, nil
}

func (s *Service) PreviewMatchSearch(in MatchSearchPreviewInput) (MatchSearchPreviewResult, error) {
	mode := normalizeGameMode(in.GameMode)
	if in.MinStake <= 0 || in.MaxStake < in.MinStake || mode == "" {
		return MatchSearchPreviewResult{}, ErrInvalidStakeRange
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
		if !rangesOverlap(waiting.MinStake, waiting.MaxStake, in.MinStake, in.MaxStake) {
			continue
		}
		count++
	}

	return MatchSearchPreviewResult{
		MatchedUsersCount: count,
		GameMode:          mode,
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

func (s *Service) createMatchedGame(ctx context.Context, player1ID, player2ID string, stake int64, mode string, engine Engine) (string, error) {
	id, err := newGameID()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[id]; exists {
		return "", errors.New("game id collision")
	}

	session := NewSession(id, mode, player1ID, player2ID, stake, engine)
	s.trackSessionVariantLocked(session)
	s.sessions[id] = session

	if s.repository != nil {
		p2 := player2ID
		err := s.repository.CreateGame(ctx, CreateGameParams{
			GameID:            id,
			Player1ID:         player1ID,
			Player2ID:         &p2,
			Status:            string(session.Status),
			BetAmount:         stake,
			MemeMode:          mode == "meme",
			GameMode:          mode,
			FEN:               session.FEN,
			CurrentTurnUserID: session.CurrentTurnUserID,
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
		return session.Snapshot(), nil
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
	return session.Snapshot(), nil
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
		return State{}, MoveResult{}, err
	}

	cursor, err := s.variantTracker.AdvanceGame(gameID, result.Move, state.FEN)
	if err != nil {
		return State{}, MoveResult{}, err
	}
	session.SetVariantCursor(cursor.RootPositionHash, cursor.CurrentPositionHash, cursor.Ply)
	state = session.Snapshot()

	if s.repository != nil && !session.IsBotGame() {
		moveNumber := len(state.Moves)

		if err := s.repository.SaveMove(ctx, SaveMoveParams{
			GameID:      gameID,
			PlayerID:    userID,
			MoveNumber:  moveNumber,
			Move:        result.Move,
			FEN:         result.FEN,
			IsCapture:   result.IsCapture,
			IsCheck:     result.IsCheck,
			IsCheckmate: result.IsCheckmate,
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
		GameID:            state.GameID,
		Status:            state.Status,
		FEN:               state.FEN,
		CurrentTurnUserID: state.CurrentTurnUserID,
		WinnerID:          winnerID,
		FinishedAt:        finishedAt,
		FinishedReason:    finishedReason,
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
		GameID:            state.GameID,
		Status:            state.Status,
		FEN:               state.FEN,
		CurrentTurnUserID: state.CurrentTurnUserID,
		WinnerID:          nil,
		FinishedAt:        nil,
		FinishedReason:    nil,
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
