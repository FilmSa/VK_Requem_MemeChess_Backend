package game

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"time"
)

var quickGameTimeControlIDs = []string{"classic", "rapid", "blitz", "bullet"}

type QuickGameSearchInput struct {
	UserID   string
	MinStake int64
	MaxStake int64
}

type QuickGameSearchResult struct {
	Status       string `json:"status"`
	GameID       string `json:"game_id,omitempty"`
	AgreedStake  int64  `json:"agreed_stake,omitempty"`
	GameCurrency string `json:"game_currency,omitempty"`
	GameMode     string `json:"game_mode,omitempty"`
	timeControlPayload
}

type QuickGameLeaveResult struct {
	Status string `json:"status"`
}

func (s *Service) SearchQuickGame(ctx context.Context, in QuickGameSearchInput) (QuickGameSearchResult, error) {
	if in.UserID == "" || in.MinStake <= 0 || in.MaxStake < in.MinStake {
		return QuickGameSearchResult{}, ErrInvalidStakeRange
	}

	if existing, ok := s.findUserWaitingQuickRoom(in.UserID); ok {
		return buildQuickGameResultFromSession("waiting", existing), nil
	}

	candidates, err := s.listQuickGameRoomCandidates(in.UserID, in.MinStake, in.MaxStake)
	if err != nil {
		return QuickGameSearchResult{}, err
	}

	for _, gameID := range candidates {
		result, err := s.tryJoinQuickGameRoom(ctx, gameID, in.UserID, in.MinStake, in.MaxStake)
		if err == nil {
			return result, nil
		}
		if errors.Is(err, ErrGameNotFound) || errors.Is(err, ErrGameFull) ||
			errors.Is(err, ErrInviteExpired) || errors.Is(err, ErrInviteUsed) ||
			errors.Is(err, ErrInviteOwnGame) {
			continue
		}
		return QuickGameSearchResult{}, err
	}

	return s.createQuickGameRoom(ctx, in)
}

func (s *Service) LeaveQuickGameSearch(ctx context.Context, userID string) (QuickGameLeaveResult, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return QuickGameLeaveResult{Status: "idle"}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for gameID, session := range s.sessions {
		if session == nil || !session.QuickGame {
			continue
		}
		snap := session.Snapshot()
		if snap.Player1ID != userID || snap.Player2ID != "" || snap.Status != string(StatusWaiting) {
			continue
		}

		stake := snap.BetAmount
		s.variantTracker.ForgetGame(gameID)
		delete(s.sessions, gameID)

		if s.userRepo != nil && stake > 0 {
			if err := s.userRepo.AddGameCurrency(ctx, userID, stake); err != nil {
				return QuickGameLeaveResult{}, err
			}
		}

		return QuickGameLeaveResult{Status: "cancelled"}, nil
	}

	return QuickGameLeaveResult{Status: "idle"}, nil
}

func (s *Service) findUserWaitingQuickRoom(userID string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	for _, session := range s.sessions {
		if session == nil || !session.QuickGame {
			continue
		}
		snap := session.Snapshot()
		if snap.Player1ID != userID || snap.Player2ID != "" || snap.Status != string(StatusWaiting) {
			continue
		}
		if session.IsInviteExpired(now) {
			continue
		}
		return session, true
	}

	return nil, false
}

func (s *Service) listQuickGameRoomCandidates(userID string, minStake, maxStake int64) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	candidates := make([]string, 0, 8)
	for gameID, session := range s.sessions {
		if !isEligibleQuickGameTarget(session, userID, minStake, maxStake, now) {
			continue
		}
		candidates = append(candidates, gameID)
	}

	if err := shuffleStrings(candidates); err != nil {
		return nil, err
	}

	return candidates, nil
}

func isEligibleQuickGameTarget(
	session *Session,
	userID string,
	minStake, maxStake int64,
	now time.Time,
) bool {
	if session == nil || !session.QuickGame || session.IsBotGame() {
		return false
	}
	snap := session.Snapshot()
	if snap.Status != string(StatusWaiting) || snap.Player2ID != "" {
		return false
	}
	if session.IsInviteExpired(now) {
		return false
	}
	if snap.Player1ID == userID {
		return false
	}
	if snap.BetAmount <= 0 || snap.BetAmount < minStake || snap.BetAmount > maxStake {
		return false
	}
	return true
}

func (s *Service) tryJoinQuickGameRoom(
	ctx context.Context,
	gameID, userID string,
	minStake, maxStake int64,
) (QuickGameSearchResult, error) {
	s.mu.RLock()
	session, ok := s.sessions[gameID]
	if !ok || !isEligibleQuickGameTarget(session, userID, minStake, maxStake, time.Now()) {
		s.mu.RUnlock()
		return QuickGameSearchResult{}, ErrGameNotFound
	}
	stake := session.BetAmount
	s.mu.RUnlock()

	if s.userRepo != nil {
		if err := s.userRepo.ReserveGameCurrency(ctx, userID, stake); err != nil {
			return QuickGameSearchResult{}, err
		}
	}

	state, err := s.ReserveInviteSeat(ctx, gameID, userID)
	if err != nil {
		if s.userRepo != nil {
			_ = s.userRepo.AddGameCurrency(ctx, userID, stake)
		}
		return QuickGameSearchResult{}, err
	}

	return QuickGameSearchResult{
		Status:             "joined",
		GameID:             gameID,
		AgreedStake:        stake,
		GameCurrency:       "game_currency",
		GameMode:           state.GameMode,
		timeControlPayload: buildTimeControlPayloadFromState(state),
	}, nil
}

func (s *Service) createQuickGameRoom(ctx context.Context, in QuickGameSearchInput) (QuickGameSearchResult, error) {
	mode, err := randomQuickGameMode()
	if err != nil {
		return QuickGameSearchResult{}, err
	}
	timeControlID, err := randomQuickGameChoice(quickGameTimeControlIDs)
	if err != nil {
		return QuickGameSearchResult{}, err
	}
	stake, err := randomStakeInRange(in.MinStake, in.MaxStake)
	if err != nil {
		return QuickGameSearchResult{}, err
	}

	engine, err := NewChessEngineForMode(mode)
	if err != nil {
		return QuickGameSearchResult{}, err
	}

	if s.userRepo != nil {
		if err := s.userRepo.ReserveGameCurrency(ctx, in.UserID, stake); err != nil {
			return QuickGameSearchResult{}, err
		}
	}

	gameID, err := s.insertQuickGameRoom(ctx, in.UserID, mode, timeControlID, stake, engine)
	if err != nil {
		if s.userRepo != nil {
			_ = s.userRepo.AddGameCurrency(ctx, in.UserID, stake)
		}
		return QuickGameSearchResult{}, err
	}

	return QuickGameSearchResult{
		Status:             "waiting",
		GameID:             gameID,
		AgreedStake:        stake,
		GameCurrency:       "game_currency",
		GameMode:           mode,
		timeControlPayload: buildTimeControlPayloadFromPreset(timeControlID),
	}, nil
}

func (s *Service) insertQuickGameRoom(
	ctx context.Context,
	hostUserID, mode, timeControlID string,
	stake int64,
	engine Engine,
) (string, error) {
	id, err := newGameID()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[id]; exists {
		return "", errors.New("game id collision")
	}

	normalizedMode := normalizeGameMode(mode)
	if normalizedMode == "" {
		return "", ErrInvalidGameMode
	}
	normalizedTimeControlID := normalizeTimeControlID(timeControlID)
	if normalizedTimeControlID == "" || normalizedTimeControlID == TimeControlUnlimited {
		return "", ErrInvalidTimeControl
	}

	session := NewSessionWithTimeControl(id, normalizedMode, hostUserID, "", stake, engine, normalizedTimeControlID)
	session.QuickGame = true
	session.InviteExpiresAt = time.Now().Add(defaultInviteTTL)
	s.trackSessionVariantLocked(session)
	s.sessions[id] = session

	if s.repository != nil {
		err := s.repository.CreateGame(ctx, CreateGameParams{
			GameID:                 id,
			Player1ID:              hostUserID,
			Player2ID:              nil,
			Status:                 string(session.Status),
			BetAmount:              stake,
			MemeMode:               normalizedMode == GameModeMeme,
			GameMode:               normalizedMode,
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

func buildQuickGameResultFromSession(status string, session *Session) QuickGameSearchResult {
	snap := session.Snapshot()
	return QuickGameSearchResult{
		Status:             status,
		GameID:             snap.GameID,
		AgreedStake:        snap.BetAmount,
		GameCurrency:       "game_currency",
		GameMode:           snap.GameMode,
		timeControlPayload: buildTimeControlPayloadFromState(snap),
	}
}

func buildTimeControlPayloadFromState(state State) timeControlPayload {
	return buildTimeControlPayload(
		state.TimeControlID,
		state.TimeControlLabel,
		state.TimeControlBaseMs,
		state.TimeControlIncrementMs,
		state.Player1RemainingMs,
		state.Player2RemainingMs,
		state.CurrentTurnStartedAt,
	)
}

func randomQuickGameMode() (string, error) {
	categories := QuickGameModeCategories()
	categoryIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(categories))))
	if err != nil {
		return "", err
	}
	return randomQuickGameChoice(categories[categoryIndex.Int64()])
}

func randomQuickGameChoice[T ~string](choices []T) (T, error) {
	var zero T
	if len(choices) == 0 {
		return zero, errors.New("empty choice list")
	}
	index, err := rand.Int(rand.Reader, big.NewInt(int64(len(choices))))
	if err != nil {
		return zero, err
	}
	return choices[index.Int64()], nil
}

func randomStakeInRange(minStake, maxStake int64) (int64, error) {
	if minStake <= 0 || maxStake < minStake {
		return 0, ErrInvalidStakeRange
	}
	if minStake == maxStake {
		return minStake, nil
	}

	span := maxStake - minStake + 1
	offset, err := rand.Int(rand.Reader, big.NewInt(span))
	if err != nil {
		return 0, err
	}

	return minStake + offset.Int64(), nil
}

func shuffleStrings(values []string) error {
	for i := len(values) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return err
		}
		idx := int(j.Int64())
		values[i], values[idx] = values[idx], values[i]
	}
	return nil
}
