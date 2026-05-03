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
	Number      int    `json:"number"`
	UserID      string `json:"user_id"`
	Move        string `json:"move"`
	FEN         string `json:"fen"`
	IsCapture   bool   `json:"is_capture"`
	IsCheck     bool   `json:"is_check"`
	IsCheckmate bool   `json:"is_checkmate"`
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

	InitialFEN          string `json:"initial_fen"`
	FEN                 string `json:"fen"`
	LastMove            string `json:"last_move"`
	WinnerID            string `json:"winner_id,omitempty"`
	FinishedReason      string `json:"finished_reason,omitempty"`
	RootPositionHash    string `json:"root_position_hash"`
	CurrentPositionHash string `json:"current_position_hash"`
	VariantPly          int    `json:"variant_ply"`
	InviteExpiresAt     time.Time
	BotGame             bool   `json:"bot_game,omitempty"`
	BotDifficulty       string `json:"bot_difficulty,omitempty"`

	DrawOfferedBy string    `json:"draw_offered_by,omitempty"`
	DrawOfferedAt time.Time `json:"draw_offered_at,omitempty"`

	Moves []Move `json:"moves"`

	engine Engine
}

func NewSession(gameID, gameMode, player1ID, player2ID string, betAmount int64, engine Engine) *Session {
	return &Session{
		GameID:            gameID,
		GameMode:          normalizeGameMode(gameMode),
		Player1ID:         player1ID,
		Player2ID:         player2ID,
		Status:            StatusWaiting,
		CurrentTurnUserID: player1ID,
		BetAmount:         betAmount,
		InitialFEN:        engine.CurrentFEN(),
		FEN:               engine.CurrentFEN(),
		Moves:             make([]Move, 0, 128),
		engine:            engine,
	}
}

func NewBotSession(gameID, gameMode, player1ID, difficulty string, engine Engine) *Session {
	session := NewSession(gameID, gameMode, player1ID, botUserID, 0, engine)
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

	moves := make([]Move, len(s.Moves))
	copy(moves, s.Moves)

	return State{
		GameID:              s.GameID,
		GameMode:            s.GameMode,
		Player1ID:           s.Player1ID,
		Player2ID:           s.Player2ID,
		Player1Connected:    s.Player1Connected,
		Player2Connected:    s.Player2Connected,
		Status:              string(s.Status),
		CurrentTurnUserID:   s.CurrentTurnUserID,
		BetAmount:           s.BetAmount,
		DrawOfferedBy:       s.DrawOfferedBy,
		DrawOfferedAt:       s.DrawOfferedAt,
		InitialFEN:          s.InitialFEN,
		FEN:                 s.FEN,
		LastMove:            s.LastMove,
		WinnerID:            s.WinnerID,
		FinishedReason:      s.FinishedReason,
		RootPositionHash:    s.RootPositionHash,
		CurrentPositionHash: s.CurrentPositionHash,
		VariantPly:          s.VariantPly,
		BotGame:             s.BotGame,
		BotDifficulty:       s.BotDifficulty,
		LegalMoves:          s.legalMovesLocked(),
		Moves:               moves,
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
	} else {
		if s.CurrentTurnUserID == s.Player1ID {
			s.CurrentTurnUserID = s.Player2ID
		} else {
			s.CurrentTurnUserID = s.Player1ID
		}
	}

	moves := make([]Move, len(s.Moves))
	copy(moves, s.Moves)

	return State{
		GameID:              s.GameID,
		GameMode:            s.GameMode,
		Player1ID:           s.Player1ID,
		Player2ID:           s.Player2ID,
		Player1Connected:    s.Player1Connected,
		Player2Connected:    s.Player2Connected,
		Status:              string(s.Status),
		CurrentTurnUserID:   s.CurrentTurnUserID,
		BetAmount:           s.BetAmount,
		DrawOfferedBy:       s.DrawOfferedBy,
		DrawOfferedAt:       s.DrawOfferedAt,
		InitialFEN:          s.InitialFEN,
		FEN:                 s.FEN,
		LastMove:            s.LastMove,
		WinnerID:            s.WinnerID,
		FinishedReason:      s.FinishedReason,
		RootPositionHash:    s.RootPositionHash,
		CurrentPositionHash: s.CurrentPositionHash,
		VariantPly:          s.VariantPly,
		BotGame:             s.BotGame,
		BotDifficulty:       s.BotDifficulty,
		LegalMoves:          s.legalMovesLocked(),
		Moves:               moves,
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
	s.DrawOfferedBy = ""
	s.DrawOfferedAt = time.Time{}

	return s.snapshotLocked(), nil
}

func (s *Session) snapshotLocked() State {
	moves := make([]Move, len(s.Moves))
	copy(moves, s.Moves)

	return State{
		GameID:              s.GameID,
		GameMode:            s.GameMode,
		Player1ID:           s.Player1ID,
		Player2ID:           s.Player2ID,
		Player1Connected:    s.Player1Connected,
		Player2Connected:    s.Player2Connected,
		Status:              string(s.Status),
		CurrentTurnUserID:   s.CurrentTurnUserID,
		BetAmount:           s.BetAmount,
		DrawOfferedBy:       s.DrawOfferedBy,
		DrawOfferedAt:       s.DrawOfferedAt,
		InitialFEN:          s.InitialFEN,
		FEN:                 s.FEN,
		LastMove:            s.LastMove,
		WinnerID:            s.WinnerID,
		FinishedReason:      s.FinishedReason,
		RootPositionHash:    s.RootPositionHash,
		CurrentPositionHash: s.CurrentPositionHash,
		VariantPly:          s.VariantPly,
		BotGame:             s.BotGame,
		BotDifficulty:       s.BotDifficulty,
		LegalMoves:          s.legalMovesLocked(),
		Moves:               moves,
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

func (s *Session) FinishDraw(reason string) State {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Status = StatusFinished
	s.WinnerID = ""
	s.FinishedReason = reason
	s.DrawOfferedBy = ""
	s.DrawOfferedAt = time.Time{}

	return s.snapshotLocked()
}
