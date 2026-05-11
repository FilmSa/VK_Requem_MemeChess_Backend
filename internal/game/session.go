package game

import (
	"sync"
	"time"
)

type Status string

const (
	StatusWaiting  Status = "waiting"
	StatusActive   Status = "active"
	StatusFinished Status = "finished"
)

type Move struct {
	Number      int          `json:"number"`
	UserID      string       `json:"user_id"`
	Move        string       `json:"move"`
	FEN         string       `json:"fen"`
	IsCapture   bool         `json:"is_capture"`
	IsCheck     bool         `json:"is_check"`
	IsCheckmate bool         `json:"is_checkmate"`
	Effects     []MoveEffect `json:"effects,omitempty"`
}

type Session struct {
	mu sync.RWMutex

	GameID   string `json:"game_id"`
	GameMode string `json:"game_mode"`

	Player1ID string `json:"player1_id"`
	Player2ID string `json:"player2_id"`

	Player1Connected bool `json:"player1_connected"`
	Player2Connected bool `json:"player2_connected"`

	Status Status `json:"status"`

	CurrentTurnUserID string `json:"current_turn_user_id"`

	BetAmount int64 `json:"bet_amount,omitempty"`

	TimeControlID        string    `json:"time_control_id,omitempty"`
	TimeControlLabel     string    `json:"time_control_label,omitempty"`
	TimeControlBaseMs    int64     `json:"time_control_base_ms,omitempty"`
	TimeControlIncrement int64     `json:"time_control_increment_ms,omitempty"`
	Player1RemainingMs   int64     `json:"player1_remaining_ms,omitempty"`
	Player2RemainingMs   int64     `json:"player2_remaining_ms,omitempty"`
	CurrentTurnStartedAt time.Time `json:"current_turn_started_at,omitempty"`

	InitialFEN          string       `json:"initial_fen"`
	FEN                 string       `json:"fen"`
	LastMove            string       `json:"last_move"`
	LastMoveEffects     []MoveEffect `json:"last_move_effects,omitempty"`
	WinnerID            string       `json:"winner_id,omitempty"`
	FinishedReason      string       `json:"finished_reason,omitempty"`
	RootPositionHash    string       `json:"root_position_hash"`
	CurrentPositionHash string       `json:"current_position_hash"`
	VariantPly          int          `json:"variant_ply"`
	InviteExpiresAt     time.Time
	BotGame             bool   `json:"bot_game,omitempty"`
	BotDifficulty       string `json:"bot_difficulty,omitempty"`

	DrawOfferedBy string    `json:"draw_offered_by,omitempty"`
	DrawOfferedAt time.Time `json:"draw_offered_at,omitempty"`

	Moves []Move `json:"moves"`

	engine Engine
}

func NewSession(gameID, gameMode, player1ID, player2ID string, betAmount int64, engine Engine) *Session {
	return NewSessionWithTimeControl(gameID, gameMode, player1ID, player2ID, betAmount, engine, TimeControlUnlimited)
}

func NewSessionWithTimeControl(
	gameID,
	gameMode,
	player1ID,
	player2ID string,
	betAmount int64,
	engine Engine,
	timeControlID string,
) *Session {
	timeControl, ok := resolveTimeControl(timeControlID)
	if !ok {
		timeControl = TimeControl{ID: TimeControlUnlimited}
	}

	return &Session{
		GameID:               gameID,
		GameMode:             normalizeGameMode(gameMode),
		Player1ID:            player1ID,
		Player2ID:            player2ID,
		Status:               StatusWaiting,
		CurrentTurnUserID:    player1ID,
		BetAmount:            betAmount,
		TimeControlID:        timeControl.ID,
		TimeControlLabel:     timeControl.Label,
		TimeControlBaseMs:    durationToMilliseconds(timeControl.Base),
		TimeControlIncrement: durationToMilliseconds(timeControl.Increment),
		Player1RemainingMs:   durationToMilliseconds(timeControl.Base),
		Player2RemainingMs:   durationToMilliseconds(timeControl.Base),
		InitialFEN:           engine.CurrentFEN(),
		FEN:                  engine.CurrentFEN(),
		Moves:                make([]Move, 0, 128),
		engine:               engine,
	}
}

func NewBotSession(gameID, gameMode, player1ID, difficulty string, engine Engine) *Session {
	session := NewSessionWithTimeControl(gameID, gameMode, player1ID, botUserID, 0, engine, TimeControlUnlimited)
	session.Player2Connected = true
	session.Status = StatusActive
	session.BotGame = true
	session.BotDifficulty = difficulty
	return session
}

func (s *Session) HasPlayer(userID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if userID == s.Player1ID {
		return true
	}
	if s.Player2ID != "" && userID == s.Player2ID {
		return true
	}
	return false
}

func (s *Session) InviteDeadline() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.InviteExpiresAt
}

func (s *Session) IsBotGame() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.BotGame
}

func (s *Session) IsInviteExpired(now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return !s.InviteExpiresAt.IsZero() && now.After(s.InviteExpiresAt) && s.Player2ID == ""
}

func (s *Session) ReserveInviteSeat(userID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userID == s.Player1ID {
		return ErrInviteOwnGame
	}
	if s.Player2ID == userID {
		return nil
	}
	if !s.InviteExpiresAt.IsZero() && now.After(s.InviteExpiresAt) && s.Player2ID == "" {
		return ErrInviteExpired
	}
	if s.Player2ID != "" {
		return ErrInviteUsed
	}

	s.Player2ID = userID
	return nil
}

func (s *Session) AssignPlayer2(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userID == s.Player1ID {
		return ErrForbidden
	}
	if s.Player2ID != "" {
		return ErrGameFull
	}
	s.Player2ID = userID
	return nil
}

func (s *Session) RollbackPlayer2If(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Player2ID == userID {
		s.Player2ID = ""
	}
}

func (s *Session) SetConnected(userID string, connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userID == s.Player1ID {
		s.Player1Connected = connected
	}
	if userID == s.Player2ID {
		s.Player2Connected = connected
	}

	if s.Player1Connected && s.Player2Connected && s.Status == StatusWaiting {
		s.Status = StatusActive
	}
}

func (s *Session) Snapshot() State {
	s.mu.RLock()
	defer s.mu.RUnlock()

	moves := cloneMoves(s.Moves)

	return State{
		GameID:                 s.GameID,
		GameMode:               s.GameMode,
		Player1ID:              s.Player1ID,
		Player2ID:              s.Player2ID,
		Player1Connected:       s.Player1Connected,
		Player2Connected:       s.Player2Connected,
		Status:                 string(s.Status),
		CurrentTurnUserID:      s.CurrentTurnUserID,
		BetAmount:              s.BetAmount,
		TimeControlID:          s.TimeControlID,
		TimeControlLabel:       s.TimeControlLabel,
		TimeControlBaseMs:      s.TimeControlBaseMs,
		TimeControlIncrementMs: s.TimeControlIncrement,
		Player1RemainingMs:     s.currentRemainingMsLocked(s.Player1ID, time.Now()),
		Player2RemainingMs:     s.currentRemainingMsLocked(s.Player2ID, time.Now()),
		CurrentTurnStartedAt:   s.CurrentTurnStartedAt,
		DrawOfferedBy:          s.DrawOfferedBy,
		DrawOfferedAt:          s.DrawOfferedAt,
		InitialFEN:             s.InitialFEN,
		FEN:                    s.FEN,
		LastMove:               s.LastMove,
		LastMoveEffects:        cloneEffects(lastMoveEffects(s.Moves)),
		WinnerID:               s.WinnerID,
		FinishedReason:         s.FinishedReason,
		RootPositionHash:       s.RootPositionHash,
		CurrentPositionHash:    s.CurrentPositionHash,
		VariantPly:             s.VariantPly,
		BotGame:                s.BotGame,
		BotDifficulty:          s.BotDifficulty,
		LegalMoves:             s.legalMovesLocked(),
		Moves:                  moves,
	}
}

func (s *Session) SetVariantCursor(rootHash, currentHash string, ply int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.RootPositionHash = rootHash
	s.CurrentPositionHash = currentHash
	s.VariantPly = ply
}

func (s *Session) ApplyMove(userID, move string) (State, MoveResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Status == StatusFinished {
		return State{}, MoveResult{}, ErrGameFinished
	}
	if s.Status != StatusActive {
		return State{}, MoveResult{}, ErrGameNotActive
	}
	if userID != s.CurrentTurnUserID {
		return State{}, MoveResult{}, ErrNotYourTurn
	}
	if s.hasTimedClockLocked() {
		now := time.Now()
		if !s.CurrentTurnStartedAt.IsZero() {
			remaining := s.currentRemainingDurationLocked(userID, now)
			if remaining <= 0 {
				s.setRemainingDurationLocked(userID, 0)
				timedOutState := s.finishTimeoutLocked(userID)
				return timedOutState, MoveResult{}, ErrTimeExpired
			}

			s.setRemainingDurationLocked(userID, remaining+s.timeIncrementLocked())
		}
	}

	result, err := s.engine.ApplyMove(move)
	if err != nil {
		return State{}, MoveResult{}, ErrInvalidMove
	}

	nextMove := Move{
		Number:      len(s.Moves) + 1,
		UserID:      userID,
		Move:        result.Move,
		FEN:         result.FEN,
		IsCapture:   result.IsCapture,
		IsCheck:     result.IsCheck,
		IsCheckmate: result.IsCheckmate,
		Effects:     cloneEffects(result.Effects),
	}

	s.Moves = append(s.Moves, nextMove)
	s.FEN = result.FEN
	s.LastMove = result.Move
	s.DrawOfferedBy = ""
	s.DrawOfferedAt = time.Time{}

	if result.IsCheckmate {
		s.Status = StatusFinished
		s.WinnerID = userID
		s.FinishedReason = "checkmate"
		s.CurrentTurnStartedAt = time.Time{}
	} else {
		if s.CurrentTurnUserID == s.Player1ID {
			s.CurrentTurnUserID = s.Player2ID
		} else {
			s.CurrentTurnUserID = s.Player1ID
		}
		if s.hasTimedClockLocked() && len(s.Moves) >= 2 {
			s.CurrentTurnStartedAt = time.Now()
		} else {
			s.CurrentTurnStartedAt = time.Time{}
		}
	}

	moves := cloneMoves(s.Moves)

	return State{
		GameID:                 s.GameID,
		GameMode:               s.GameMode,
		Player1ID:              s.Player1ID,
		Player2ID:              s.Player2ID,
		Player1Connected:       s.Player1Connected,
		Player2Connected:       s.Player2Connected,
		Status:                 string(s.Status),
		CurrentTurnUserID:      s.CurrentTurnUserID,
		BetAmount:              s.BetAmount,
		TimeControlID:          s.TimeControlID,
		TimeControlLabel:       s.TimeControlLabel,
		TimeControlBaseMs:      s.TimeControlBaseMs,
		TimeControlIncrementMs: s.TimeControlIncrement,
		Player1RemainingMs:     s.currentRemainingMsLocked(s.Player1ID, time.Now()),
		Player2RemainingMs:     s.currentRemainingMsLocked(s.Player2ID, time.Now()),
		CurrentTurnStartedAt:   s.CurrentTurnStartedAt,
		DrawOfferedBy:          s.DrawOfferedBy,
		DrawOfferedAt:          s.DrawOfferedAt,
		InitialFEN:             s.InitialFEN,
		FEN:                    s.FEN,
		LastMove:               s.LastMove,
		LastMoveEffects:        cloneEffects(result.Effects),
		WinnerID:               s.WinnerID,
		FinishedReason:         s.FinishedReason,
		RootPositionHash:       s.RootPositionHash,
		CurrentPositionHash:    s.CurrentPositionHash,
		VariantPly:             s.VariantPly,
		BotGame:                s.BotGame,
		BotDifficulty:          s.BotDifficulty,
		LegalMoves:             s.legalMovesLocked(),
		Moves:                  moves,
	}, result, nil
}

func (s *Session) Resign(userID string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Status == StatusFinished {
		return State{}, ErrGameFinished
	}
	if s.Status != StatusActive {
		return State{}, ErrGameNotActive
	}
	if userID != s.Player1ID && userID != s.Player2ID {
		return State{}, ErrForbidden
	}

	winner := s.Player1ID
	if userID == s.Player1ID {
		winner = s.Player2ID
	}

	s.Status = StatusFinished
	s.WinnerID = winner
	s.FinishedReason = "resign"
	s.CurrentTurnStartedAt = time.Time{}
	s.DrawOfferedBy = ""
	s.DrawOfferedAt = time.Time{}

	return s.snapshotLocked(), nil
}

func (s *Session) OfferDraw(userID string, now time.Time) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Status == StatusFinished {
		return State{}, ErrGameFinished
	}
	if s.Status != StatusActive {
		return State{}, ErrGameNotActive
	}
	if userID != s.Player1ID && userID != s.Player2ID {
		return State{}, ErrForbidden
	}

	if s.DrawOfferedBy == userID {
		return s.snapshotLocked(), nil
	}

	s.DrawOfferedBy = userID
	s.DrawOfferedAt = now
	return s.snapshotLocked(), nil
}

func (s *Session) DeclineDraw(userID string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Status == StatusFinished {
		return State{}, ErrGameFinished
	}
	if s.Status != StatusActive {
		return State{}, ErrGameNotActive
	}
	if userID != s.Player1ID && userID != s.Player2ID {
		return State{}, ErrForbidden
	}
	if s.DrawOfferedBy == "" {
		return s.snapshotLocked(), nil
	}
	if s.DrawOfferedBy == userID {
		return State{}, ErrForbidden
	}

	s.DrawOfferedBy = ""
	s.DrawOfferedAt = time.Time{}
	return s.snapshotLocked(), nil
}

func (s *Session) AcceptDraw(userID string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Status == StatusFinished {
		return State{}, ErrGameFinished
	}
	if s.Status != StatusActive {
		return State{}, ErrGameNotActive
	}
	if userID != s.Player1ID && userID != s.Player2ID {
		return State{}, ErrForbidden
	}
	if s.DrawOfferedBy == "" {
		return State{}, ErrForbidden
	}
	if s.DrawOfferedBy == userID {
		return State{}, ErrForbidden
	}

	s.Status = StatusFinished
	s.WinnerID = ""
	s.FinishedReason = "draw_agreed"
	s.CurrentTurnStartedAt = time.Time{}
	s.DrawOfferedBy = ""
	s.DrawOfferedAt = time.Time{}

	return s.snapshotLocked(), nil
}

func (s *Session) snapshotLocked() State {
	moves := cloneMoves(s.Moves)

	return State{
		GameID:                 s.GameID,
		GameMode:               s.GameMode,
		Player1ID:              s.Player1ID,
		Player2ID:              s.Player2ID,
		Player1Connected:       s.Player1Connected,
		Player2Connected:       s.Player2Connected,
		Status:                 string(s.Status),
		CurrentTurnUserID:      s.CurrentTurnUserID,
		BetAmount:              s.BetAmount,
		TimeControlID:          s.TimeControlID,
		TimeControlLabel:       s.TimeControlLabel,
		TimeControlBaseMs:      s.TimeControlBaseMs,
		TimeControlIncrementMs: s.TimeControlIncrement,
		Player1RemainingMs:     s.currentRemainingMsLocked(s.Player1ID, time.Now()),
		Player2RemainingMs:     s.currentRemainingMsLocked(s.Player2ID, time.Now()),
		CurrentTurnStartedAt:   s.CurrentTurnStartedAt,
		DrawOfferedBy:          s.DrawOfferedBy,
		DrawOfferedAt:          s.DrawOfferedAt,
		InitialFEN:             s.InitialFEN,
		FEN:                    s.FEN,
		LastMove:               s.LastMove,
		LastMoveEffects:        cloneEffects(lastMoveEffects(s.Moves)),
		WinnerID:               s.WinnerID,
		FinishedReason:         s.FinishedReason,
		RootPositionHash:       s.RootPositionHash,
		CurrentPositionHash:    s.CurrentPositionHash,
		VariantPly:             s.VariantPly,
		BotGame:                s.BotGame,
		BotDifficulty:          s.BotDifficulty,
		LegalMoves:             s.legalMovesLocked(),
		Moves:                  moves,
	}
}

func (s *Session) legalMovesLocked() []string {
	type legalMoveProvider interface {
		LegalMoves() []string
	}

	provider, ok := s.engine.(legalMoveProvider)
	if !ok {
		return nil
	}

	moves := provider.LegalMoves()
	if len(moves) == 0 {
		return nil
	}

	cloned := make([]string, len(moves))
	copy(cloned, moves)
	return cloned
}

func lastMoveEffects(moves []Move) []MoveEffect {
	if len(moves) == 0 {
		return nil
	}
	return moves[len(moves)-1].Effects
}

func cloneMoves(in []Move) []Move {
	if len(in) == 0 {
		return nil
	}

	out := make([]Move, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Effects = cloneEffects(in[i].Effects)
	}
	return out
}

func (s *Session) FinishDraw(reason string) State {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Status = StatusFinished
	s.WinnerID = ""
	s.FinishedReason = reason
	s.CurrentTurnStartedAt = time.Time{}
	s.DrawOfferedBy = ""
	s.DrawOfferedAt = time.Time{}

	return s.snapshotLocked()
}

func (s *Session) Timeout(now time.Time) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Status == StatusFinished {
		return State{}, ErrGameFinished
	}
	if s.Status != StatusActive {
		return State{}, ErrGameNotActive
	}
	if !s.hasTimedClockLocked() {
		return State{}, ErrUntimedGame
	}
	if s.CurrentTurnUserID == "" {
		return State{}, ErrClockStillRunning
	}
	if s.currentRemainingDurationLocked(s.CurrentTurnUserID, now) > 0 {
		return State{}, ErrClockStillRunning
	}

	s.setRemainingDurationLocked(s.CurrentTurnUserID, 0)
	return s.finishTimeoutLocked(s.CurrentTurnUserID), nil
}

func (s *Session) hasTimedClockLocked() bool {
	return s.TimeControlID != "" &&
		s.TimeControlID != TimeControlUnlimited &&
		s.TimeControlBaseMs > 0
}

func (s *Session) timeIncrementLocked() time.Duration {
	return time.Duration(s.TimeControlIncrement) * time.Millisecond
}

func (s *Session) currentRemainingMsLocked(userID string, now time.Time) int64 {
	return durationToMilliseconds(s.currentRemainingDurationLocked(userID, now))
}

func (s *Session) currentRemainingDurationLocked(userID string, now time.Time) time.Duration {
	if userID == "" {
		return 0
	}

	var remainingMs int64
	switch userID {
	case s.Player1ID:
		remainingMs = s.Player1RemainingMs
	case s.Player2ID:
		remainingMs = s.Player2RemainingMs
	default:
		return 0
	}

	remaining := time.Duration(remainingMs) * time.Millisecond
	if !s.hasTimedClockLocked() || s.Status != StatusActive || s.CurrentTurnUserID != userID || s.CurrentTurnStartedAt.IsZero() {
		if remaining < 0 {
			return 0
		}
		return remaining
	}

	elapsed := now.Sub(s.CurrentTurnStartedAt)
	if elapsed <= 0 {
		return remaining
	}
	if elapsed >= remaining {
		return 0
	}
	return remaining - elapsed
}

func (s *Session) setRemainingDurationLocked(userID string, value time.Duration) {
	remainingMs := durationToMilliseconds(value)
	switch userID {
	case s.Player1ID:
		s.Player1RemainingMs = remainingMs
	case s.Player2ID:
		s.Player2RemainingMs = remainingMs
	}
}

func (s *Session) finishTimeoutLocked(loserID string) State {
	winnerID := s.Player1ID
	if loserID == s.Player1ID {
		winnerID = s.Player2ID
	}

	s.Status = StatusFinished
	s.WinnerID = winnerID
	s.FinishedReason = "timeout"
	s.CurrentTurnStartedAt = time.Time{}
	s.DrawOfferedBy = ""
	s.DrawOfferedAt = time.Time{}

	return s.snapshotLocked()
}
