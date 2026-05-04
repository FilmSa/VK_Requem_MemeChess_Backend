package game

import (
	"context"
	"testing"
	"time"
)

func newTestServiceWithGame() *Service {
	svc := NewService(nil)

	_, err := svc.CreateGame(context.Background(), "game-123", "user1", "user2", NewChessEngine())
	if err != nil {
		panic(err)
	}

	return svc
}

func activateGame(t *testing.T, svc *Service) {
	t.Helper()

	_, err := svc.JoinGame(context.Background(), "game-123", "user1")
	if err != nil {
		t.Fatalf("user1 failed to join: %v", err)
	}

	_, err = svc.JoinGame(context.Background(), "game-123", "user2")
	if err != nil {
		t.Fatalf("user2 failed to join: %v", err)
	}
}

func TestJoinGame(t *testing.T) {
	svc := newTestServiceWithGame()

	state1, err := svc.JoinGame(context.Background(), "game-123", "user1")
	if err != nil {
		t.Fatalf("user1 join failed: %v", err)
	}

	if !state1.Player1Connected {
		t.Fatalf("expected player1 to be connected")
	}

	if state1.Status != string(StatusWaiting) {
		t.Fatalf("expected status waiting after first join, got %s", state1.Status)
	}

	state2, err := svc.JoinGame(context.Background(), "game-123", "user2")
	if err != nil {
		t.Fatalf("user2 join failed: %v", err)
	}

	if !state2.Player2Connected {
		t.Fatalf("expected player2 to be connected")
	}

	if state2.Status != string(StatusActive) {
		t.Fatalf("expected status active after both joined, got %s", state2.Status)
	}
}

func TestJoinGame_Forbidden(t *testing.T) {
	svc := newTestServiceWithGame()

	_, err := svc.JoinGame(context.Background(), "game-123", "intruder")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err != ErrGameFull {
		t.Fatalf("expected ErrGameFull for third party when room is full, got %v", err)
	}
}

func TestInviteSecondPlayerJoins(t *testing.T) {
	svc := NewService(nil)

	gameID, err := svc.CreateInviteGame(context.Background(), "host", NewChessEngine())
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}

	_, err = svc.JoinGame(context.Background(), gameID, "host")
	if err != nil {
		t.Fatalf("host join: %v", err)
	}

	st, err := svc.JoinGame(context.Background(), gameID, "guest")
	if err != nil {
		t.Fatalf("guest join: %v", err)
	}
	if st.Player2ID != "guest" {
		t.Fatalf("expected player2 guest, got %q", st.Player2ID)
	}
	if st.Status != string(StatusActive) {
		t.Fatalf("expected active after both connected, got %s", st.Status)
	}
}

func TestInviteThirdPlayerRejected(t *testing.T) {
	svc := NewService(nil)

	gameID, err := svc.CreateInviteGame(context.Background(), "host", NewChessEngine())
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	_, _ = svc.JoinGame(context.Background(), gameID, "host")
	_, _ = svc.JoinGame(context.Background(), gameID, "guest")

	_, err = svc.JoinGame(context.Background(), gameID, "intruder")
	if err != ErrGameFull {
		t.Fatalf("expected ErrGameFull, got %v", err)
	}
}

func TestReserveInviteSeatKeepsGameWaitingUntilSocketJoin(t *testing.T) {
	svc := NewService(nil)

	gameID, err := svc.CreateInviteGame(context.Background(), "host", NewChessEngine())
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}

	hostState, err := svc.JoinGame(context.Background(), gameID, "host")
	if err != nil {
		t.Fatalf("host join: %v", err)
	}
	if hostState.Status != string(StatusWaiting) {
		t.Fatalf("expected waiting while host is alone, got %s", hostState.Status)
	}

	reservedState, err := svc.ReserveInviteSeat(context.Background(), gameID, "guest")
	if err != nil {
		t.Fatalf("reserve invite seat: %v", err)
	}
	if reservedState.Player2ID != "guest" {
		t.Fatalf("expected reserved guest in second seat, got %q", reservedState.Player2ID)
	}
	if reservedState.Status != string(StatusWaiting) {
		t.Fatalf("expected waiting after reserve, got %s", reservedState.Status)
	}

	joinedState, err := svc.JoinGame(context.Background(), gameID, "guest")
	if err != nil {
		t.Fatalf("guest socket join: %v", err)
	}
	if joinedState.Status != string(StatusActive) {
		t.Fatalf("expected active after guest websocket join, got %s", joinedState.Status)
	}
}

func TestReserveInviteSeatRejectsExpiredInvite(t *testing.T) {
	svc := NewService(nil)

	gameID, err := svc.CreateInviteGame(context.Background(), "host", NewChessEngine())
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}

	session, ok := svc.GetSession(gameID)
	if !ok {
		t.Fatal("expected invite session")
	}

	session.mu.Lock()
	session.InviteExpiresAt = time.Now().Add(-time.Minute)
	session.mu.Unlock()

	_, err = svc.ReserveInviteSeat(context.Background(), gameID, "guest")
	if err != ErrInviteExpired {
		t.Fatalf("expected ErrInviteExpired, got %v", err)
	}

	_, err = svc.JoinGame(context.Background(), gameID, "guest")
	if err != ErrInviteExpired {
		t.Fatalf("expected websocket join to fail with ErrInviteExpired, got %v", err)
	}
}

func TestMoveTurnOrder(t *testing.T) {
	svc := newTestServiceWithGame()
	activateGame(t, svc)

	_, _, err := svc.MakeMove(context.Background(), "game-123", "user2", "e7e5")
	if err == nil {
		t.Fatal("expected not your turn error, got nil")
	}

	if err != ErrNotYourTurn {
		t.Fatalf("expected ErrNotYourTurn, got %v", err)
	}

	state, result, err := svc.MakeMove(context.Background(), "game-123", "user1", "e2e4")
	if err != nil {
		t.Fatalf("expected valid move, got error: %v", err)
	}

	if result.Move != "e2e4" {
		t.Fatalf("expected move e2e4, got %s", result.Move)
	}

	if state.CurrentTurnUserID != "user2" {
		t.Fatalf("expected current turn to switch to user2, got %s", state.CurrentTurnUserID)
	}
}

func TestInvalidMove(t *testing.T) {
	svc := newTestServiceWithGame()
	activateGame(t, svc)

	_, _, err := svc.MakeMove(context.Background(), "game-123", "user1", "e2e5")
	if err == nil {
		t.Fatal("expected invalid move error, got nil")
	}

	if err != ErrInvalidMove {
		t.Fatalf("expected ErrInvalidMove, got %v", err)
	}
}

func TestCheckmate_FoolsMate(t *testing.T) {
	svc := newTestServiceWithGame()
	activateGame(t, svc)

	_, _, err := svc.MakeMove(context.Background(), "game-123", "user1", "f2f3")
	if err != nil {
		t.Fatalf("move f2f3 failed: %v", err)
	}

	_, _, err = svc.MakeMove(context.Background(), "game-123", "user2", "e7e5")
	if err != nil {
		t.Fatalf("move e7e5 failed: %v", err)
	}

	_, _, err = svc.MakeMove(context.Background(), "game-123", "user1", "g2g4")
	if err != nil {
		t.Fatalf("move g2g4 failed: %v", err)
	}

	state, result, err := svc.MakeMove(context.Background(), "game-123", "user2", "d8h4")
	if err != nil {
		t.Fatalf("move d8h4 failed: %v", err)
	}

	if !result.IsCheckmate {
		t.Fatal("expected checkmate to be detected")
	}

	if state.Status != string(StatusFinished) {
		t.Fatalf("expected game status finished, got %s", state.Status)
	}

	if state.WinnerID != "user2" {
		t.Fatalf("expected winner user2, got %s", state.WinnerID)
	}

	if state.FinishedReason != "checkmate" {
		t.Fatalf("expected finished reason checkmate, got %s", state.FinishedReason)
	}
}

func TestTimeout_FinishesTimedGame(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.CreateGameWithTimeControl(
		context.Background(),
		"timed-game",
		GameModeClassic,
		"user1",
		"user2",
		NewChessEngine(),
		"bullet",
	)
	if err != nil {
		t.Fatalf("create timed game: %v", err)
	}

	_, err = svc.JoinGame(context.Background(), "timed-game", "user1")
	if err != nil {
		t.Fatalf("join user1: %v", err)
	}
	_, err = svc.JoinGame(context.Background(), "timed-game", "user2")
	if err != nil {
		t.Fatalf("join user2: %v", err)
	}

	session, ok := svc.GetSession("timed-game")
	if !ok {
		t.Fatal("expected timed game session")
	}

	session.mu.Lock()
	session.Player1RemainingMs = 0
	session.CurrentTurnStartedAt = time.Now().Add(-2 * time.Second)
	session.mu.Unlock()

	state, err := svc.Timeout(context.Background(), "timed-game", "user2")
	if err != nil {
		t.Fatalf("timeout failed: %v", err)
	}
	if state.Status != string(StatusFinished) {
		t.Fatalf("expected finished status, got %q", state.Status)
	}
	if state.WinnerID != "user2" {
		t.Fatalf("expected user2 to win on timeout, got %q", state.WinnerID)
	}
	if state.FinishedReason != "timeout" {
		t.Fatalf("expected timeout reason, got %q", state.FinishedReason)
	}
}

func TestTimedGameClockStartsAfterSecondPlayerFirstMove(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.CreateGameWithTimeControl(
		context.Background(),
		"delayed-clock-game",
		GameModeClassic,
		"user1",
		"user2",
		NewChessEngine(),
		"rapid",
	)
	if err != nil {
		t.Fatalf("create timed game: %v", err)
	}

	_, err = svc.JoinGame(context.Background(), "delayed-clock-game", "user1")
	if err != nil {
		t.Fatalf("join user1: %v", err)
	}
	_, err = svc.JoinGame(context.Background(), "delayed-clock-game", "user2")
	if err != nil {
		t.Fatalf("join user2: %v", err)
	}

	session, ok := svc.GetSession("delayed-clock-game")
	if !ok {
		t.Fatal("expected timed game session")
	}

	initial := session.Snapshot()
	if !initial.CurrentTurnStartedAt.IsZero() {
		t.Fatal("expected clock to stay idle until the second player makes the first move")
	}
	if initial.Player1RemainingMs != 15*60*1000 {
		t.Fatalf("expected full initial time for player1, got %d", initial.Player1RemainingMs)
	}
	if initial.Player2RemainingMs != 15*60*1000 {
		t.Fatalf("expected full initial time for player2, got %d", initial.Player2RemainingMs)
	}

	time.Sleep(20 * time.Millisecond)

	afterWhiteMove, _, err := svc.MakeMove(context.Background(), "delayed-clock-game", "user1", "e2e4")
	if err != nil {
		t.Fatalf("white first move failed: %v", err)
	}
	if !afterWhiteMove.CurrentTurnStartedAt.IsZero() {
		t.Fatal("expected clock to remain idle after the first player's opening move")
	}
	if afterWhiteMove.Player1RemainingMs != initial.Player1RemainingMs {
		t.Fatalf("expected player1 time to remain unchanged before clock start, got %d", afterWhiteMove.Player1RemainingMs)
	}

	time.Sleep(20 * time.Millisecond)

	afterBlackMove, _, err := svc.MakeMove(context.Background(), "delayed-clock-game", "user2", "e7e5")
	if err != nil {
		t.Fatalf("black first move failed: %v", err)
	}
	if afterBlackMove.CurrentTurnStartedAt.IsZero() {
		t.Fatal("expected clock to start after the second player's first move")
	}
	if afterBlackMove.Player2RemainingMs != initial.Player2RemainingMs {
		t.Fatalf("expected player2 time to remain unchanged before clock start, got %d", afterBlackMove.Player2RemainingMs)
	}
}

func TestGameTracksVariantRootOnCreate(t *testing.T) {
	svc := newTestServiceWithGame()

	session, ok := svc.GetSession("game-123")
	if !ok {
		t.Fatal("expected game session to exist")
	}

	state := session.Snapshot()
	if state.RootPositionHash == "" {
		t.Fatal("expected root position hash to be initialized")
	}
	if state.CurrentPositionHash != state.RootPositionHash {
		t.Fatalf("expected current position hash to start at root, got root=%q current=%q", state.RootPositionHash, state.CurrentPositionHash)
	}
	if state.VariantPly != 0 {
		t.Fatalf("expected variant ply 0, got %d", state.VariantPly)
	}
}

func TestParallelGamesTrackIndependentVariantCursor(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.CreateGame(context.Background(), "game-1", "user1", "user2", NewChessEngine())
	if err != nil {
		t.Fatalf("create game-1: %v", err)
	}
	_, err = svc.CreateGame(context.Background(), "game-2", "user3", "user4", NewChessEngine())
	if err != nil {
		t.Fatalf("create game-2: %v", err)
	}

	_, _ = svc.JoinGame(context.Background(), "game-1", "user1")
	_, _ = svc.JoinGame(context.Background(), "game-1", "user2")
	_, _ = svc.JoinGame(context.Background(), "game-2", "user3")
	_, _ = svc.JoinGame(context.Background(), "game-2", "user4")

	state1, _, err := svc.MakeMove(context.Background(), "game-1", "user1", "e2e4")
	if err != nil {
		t.Fatalf("game-1 move: %v", err)
	}

	session2, ok := svc.GetSession("game-2")
	if !ok {
		t.Fatal("expected second game session")
	}
	state2 := session2.Snapshot()

	if state1.CurrentPositionHash == state1.RootPositionHash {
		t.Fatal("expected first game cursor to move away from root")
	}
	if state2.CurrentPositionHash != state2.RootPositionHash {
		t.Fatalf("expected second game to stay on root, got root=%q current=%q", state2.RootPositionHash, state2.CurrentPositionHash)
	}
	if state1.CurrentPositionHash == state2.CurrentPositionHash {
		t.Fatal("expected parallel games with different progress to have different current position hashes")
	}
}

func TestParallelGamesShareCommonVariantNodeForSamePosition(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.CreateGame(context.Background(), "game-a", "user1", "user2", NewChessEngine())
	if err != nil {
		t.Fatalf("create game-a: %v", err)
	}
	_, err = svc.CreateGame(context.Background(), "game-b", "user3", "user4", NewChessEngine())
	if err != nil {
		t.Fatalf("create game-b: %v", err)
	}

	_, _ = svc.JoinGame(context.Background(), "game-a", "user1")
	_, _ = svc.JoinGame(context.Background(), "game-a", "user2")
	_, _ = svc.JoinGame(context.Background(), "game-b", "user3")
	_, _ = svc.JoinGame(context.Background(), "game-b", "user4")

	stateA, _, err := svc.MakeMove(context.Background(), "game-a", "user1", "e2e4")
	if err != nil {
		t.Fatalf("game-a move: %v", err)
	}
	stateB, _, err := svc.MakeMove(context.Background(), "game-b", "user3", "e2e4")
	if err != nil {
		t.Fatalf("game-b move: %v", err)
	}

	if stateA.RootPositionHash != stateB.RootPositionHash {
		t.Fatalf("expected identical games to share root hash, got %q vs %q", stateA.RootPositionHash, stateB.RootPositionHash)
	}
	if stateA.CurrentPositionHash != stateB.CurrentPositionHash {
		t.Fatalf("expected identical resulting positions to share current hash, got %q vs %q", stateA.CurrentPositionHash, stateB.CurrentPositionHash)
	}
	if stateA.VariantPly != 1 || stateB.VariantPly != 1 {
		t.Fatalf("expected both games to be at ply 1, got %d and %d", stateA.VariantPly, stateB.VariantPly)
	}
}

func TestSearchMatch_InvalidRange(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u1",
		GameMode:      "classic",
		TimeControlID: "rapid",
		MinStake:      20,
		MaxStake:      10,
	}, NewChessEngine())
	if err != ErrInvalidStakeRange {
		t.Fatalf("expected ErrInvalidStakeRange, got %v", err)
	}
}

func TestSearchMatch_FirstPlayerQueued(t *testing.T) {
	svc := NewService(nil)

	result, err := svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u1",
		GameMode:      "classic",
		TimeControlID: "rapid",
		MinStake:      10,
		MaxStake:      50,
	}, NewChessEngine())
	if err != nil {
		t.Fatalf("search match failed: %v", err)
	}

	if result.Status != "queued" {
		t.Fatalf("expected queued status, got %q", result.Status)
	}
	if result.GameMode != "classic" {
		t.Fatalf("expected classic mode, got %q", result.GameMode)
	}
}

func TestSearchMatch_RequiresSameModeAndOverlappingRange(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u1",
		GameMode:      "meme",
		TimeControlID: "rapid",
		MinStake:      10,
		MaxStake:      30,
	}, NewChessEngine())
	if err != nil {
		t.Fatalf("queue first player: %v", err)
	}

	result, err := svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u2",
		GameMode:      "classic",
		TimeControlID: "rapid",
		MinStake:      15,
		MaxStake:      25,
	}, NewChessEngine())
	if err != nil {
		t.Fatalf("search second player: %v", err)
	}
	if result.Status != "queued" {
		t.Fatalf("expected queued when mode differs, got %q", result.Status)
	}

	result, err = svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u3",
		GameMode:      "meme",
		TimeControlID: "rapid",
		MinStake:      31,
		MaxStake:      60,
	}, NewChessEngine())
	if err != nil {
		t.Fatalf("search third player: %v", err)
	}
	if result.Status != "queued" {
		t.Fatalf("expected queued when ranges do not overlap, got %q", result.Status)
	}
}

func TestSearchMatch_RequiresSameTimeControl(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u1",
		GameMode:      "classic",
		TimeControlID: "rapid",
		MinStake:      10,
		MaxStake:      30,
	}, NewChessEngine())
	if err != nil {
		t.Fatalf("queue first player: %v", err)
	}

	result, err := svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u2",
		GameMode:      "classic",
		TimeControlID: "bullet",
		MinStake:      10,
		MaxStake:      30,
	}, NewChessEngine())
	if err != nil {
		t.Fatalf("queue second player: %v", err)
	}
	if result.Status != "queued" {
		t.Fatalf("expected queued when time control differs, got %q", result.Status)
	}
}

func TestSearchMatch_MatchesOnOverlap(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u1",
		GameMode:      "meme",
		TimeControlID: "rapid",
		MinStake:      10,
		MaxStake:      50,
	}, NewChessEngine())
	if err != nil {
		t.Fatalf("queue first player: %v", err)
	}

	result, err := svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u2",
		GameMode:      "meme",
		TimeControlID: "rapid",
		MinStake:      40,
		MaxStake:      100,
	}, NewChessEngine())
	if err != nil {
		t.Fatalf("match second player: %v", err)
	}

	if result.Status != "matched" {
		t.Fatalf("expected matched status, got %q", result.Status)
	}
	if result.GameID == "" {
		t.Fatal("expected non-empty game id")
	}
	if result.AgreedStake != 40 {
		t.Fatalf("expected agreed stake 40, got %d", result.AgreedStake)
	}
	if result.GameMode != "meme" {
		t.Fatalf("expected meme mode, got %q", result.GameMode)
	}
	if result.GameCurrency != "game_currency" {
		t.Fatalf("expected game_currency marker, got %q", result.GameCurrency)
	}
	if result.TimeControlID != "rapid" {
		t.Fatalf("expected rapid time control, got %q", result.TimeControlID)
	}

	session, ok := svc.GetSession(result.GameID)
	if !ok {
		t.Fatalf("expected created session for game %q", result.GameID)
	}
	snapshot := session.Snapshot()
	if snapshot.Player1ID != "u2" || snapshot.Player2ID != "u1" {
		t.Fatalf("unexpected players in created game: p1=%q p2=%q", snapshot.Player1ID, snapshot.Player2ID)
	}
}

func TestSearchMatch_ReturnsPendingMatchToFirstPlayer(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u1",
		GameMode:      "meme",
		TimeControlID: "rapid",
		MinStake:      10,
		MaxStake:      50,
	}, NewChessEngine())
	if err != nil {
		t.Fatalf("queue first player: %v", err)
	}

	secondResult, err := svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u2",
		GameMode:      "meme",
		TimeControlID: "rapid",
		MinStake:      40,
		MaxStake:      100,
	}, NewChessEngine())
	if err != nil {
		t.Fatalf("match second player: %v", err)
	}

	firstResult, err := svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u1",
		GameMode:      "meme",
		TimeControlID: "rapid",
		MinStake:      10,
		MaxStake:      50,
	}, NewChessEngine())
	if err != nil {
		t.Fatalf("resume first player polling: %v", err)
	}

	if firstResult.Status != "matched" {
		t.Fatalf("expected matched status for first player, got %q", firstResult.Status)
	}
	if firstResult.GameID != secondResult.GameID {
		t.Fatalf("expected same game id for both players, got first=%q second=%q", firstResult.GameID, secondResult.GameID)
	}
	if firstResult.AgreedStake != secondResult.AgreedStake {
		t.Fatalf("expected same agreed stake, got first=%d second=%d", firstResult.AgreedStake, secondResult.AgreedStake)
	}
	if firstResult.GameMode != secondResult.GameMode {
		t.Fatalf("expected same game mode, got first=%q second=%q", firstResult.GameMode, secondResult.GameMode)
	}
}

func TestLeaveMatchSearch_ClearsPendingMatch(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u1",
		GameMode:      "classic",
		TimeControlID: "rapid",
		MinStake:      10,
		MaxStake:      50,
	}, NewChessEngine())
	if err != nil {
		t.Fatalf("queue first player: %v", err)
	}

	_, err = svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u2",
		GameMode:      "classic",
		TimeControlID: "rapid",
		MinStake:      10,
		MaxStake:      50,
	}, NewChessEngine())
	if err != nil {
		t.Fatalf("match second player: %v", err)
	}

	cancelResult := svc.LeaveMatchSearch("u1")
	if cancelResult.Status != "idle" {
		t.Fatalf("expected idle when clearing pending match, got %q", cancelResult.Status)
	}

	nextResult, err := svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u1",
		GameMode:      "classic",
		TimeControlID: "rapid",
		MinStake:      10,
		MaxStake:      50,
	}, NewChessEngine())
	if err != nil {
		t.Fatalf("restart search after clearing pending match: %v", err)
	}

	if nextResult.Status != "queued" {
		t.Fatalf("expected queued after clearing pending match, got %q", nextResult.Status)
	}
}

func TestLeaveMatchSearch_RemovesQueuedPlayer(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u1",
		GameMode:      "classic",
		TimeControlID: "rapid",
		MinStake:      10,
		MaxStake:      50,
	}, NewChessEngine())
	if err != nil {
		t.Fatalf("queue player: %v", err)
	}

	result := svc.LeaveMatchSearch("u1")
	if result.Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %q", result.Status)
	}

	// If u1 was removed from queue, u2 should not be matched and should become queued.
	next, err := svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u2",
		GameMode:      "classic",
		TimeControlID: "rapid",
		MinStake:      10,
		MaxStake:      50,
	}, NewChessEngine())
	if err != nil {
		t.Fatalf("search after leave: %v", err)
	}
	if next.Status != "queued" {
		t.Fatalf("expected queued after previous player left queue, got %q", next.Status)
	}
}

func TestLeaveMatchSearch_IdleWhenNotQueued(t *testing.T) {
	svc := NewService(nil)

	result := svc.LeaveMatchSearch("unknown")
	if result.Status != "idle" {
		t.Fatalf("expected idle status, got %q", result.Status)
	}
}

func TestPreviewMatchSearch_CountsOverlappingUsersWithSameMode(t *testing.T) {
	svc := NewService(nil)

	_, _ = svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u1",
		GameMode:      "classic",
		TimeControlID: "rapid",
		MinStake:      10,
		MaxStake:      20,
	}, NewChessEngine())
	_, _ = svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u2",
		GameMode:      "classic",
		TimeControlID: "rapid",
		MinStake:      30,
		MaxStake:      40,
	}, NewChessEngine())
	_, _ = svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u3",
		GameMode:      "meme",
		TimeControlID: "rapid",
		MinStake:      10,
		MaxStake:      100,
	}, NewChessEngine())

	preview, err := svc.PreviewMatchSearch(MatchSearchPreviewInput{
		UserID:        "u4",
		GameMode:      "classic",
		TimeControlID: "rapid",
		MinStake:      15,
		MaxStake:      35,
	})
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	if preview.GameMode != "classic" {
		t.Fatalf("expected classic mode, got %q", preview.GameMode)
	}
	if preview.MatchedUsersCount != 2 {
		t.Fatalf("expected 2 matched users, got %d", preview.MatchedUsersCount)
	}
}

func TestPreviewMatchSearch_ExcludesCurrentUserQueueEntry(t *testing.T) {
	svc := NewService(nil)

	_, _ = svc.SearchMatch(context.Background(), MatchSearchInput{
		UserID:        "u1",
		GameMode:      "classic",
		TimeControlID: "rapid",
		MinStake:      10,
		MaxStake:      50,
	}, NewChessEngine())

	preview, err := svc.PreviewMatchSearch(MatchSearchPreviewInput{
		UserID:        "u1",
		GameMode:      "classic",
		TimeControlID: "rapid",
		MinStake:      10,
		MaxStake:      50,
	})
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	if preview.MatchedUsersCount != 0 {
		t.Fatalf("expected 0 matched users when only self is queued, got %d", preview.MatchedUsersCount)
	}
}

func TestPreviewMatchSearch_InvalidRange(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.PreviewMatchSearch(MatchSearchPreviewInput{
		UserID:        "u1",
		GameMode:      "classic",
		TimeControlID: "rapid",
		MinStake:      50,
		MaxStake:      10,
	})
	if err != ErrInvalidStakeRange {
		t.Fatalf("expected ErrInvalidStakeRange, got %v", err)
	}
}
