package game

import "testing"

type scriptedMove struct {
	input           string
	result          MoveResult
	legalMovesAfter []string
}

type scriptedEngine struct {
	fen        string
	legalMoves []string
	moves      []scriptedMove
	index      int
}

func (e *scriptedEngine) CurrentFEN() string {
	return e.fen
}

func (e *scriptedEngine) ApplyMove(move string) (MoveResult, error) {
	if e.index >= len(e.moves) {
		return MoveResult{}, ErrInvalidMove
	}

	next := e.moves[e.index]
	if move != next.input {
		return MoveResult{}, ErrInvalidMove
	}

	e.index++
	e.fen = next.result.FEN
	e.legalMoves = append([]string(nil), next.legalMovesAfter...)
	return next.result, nil
}

func (e *scriptedEngine) LegalMoves() []string {
	return append([]string(nil), e.legalMoves...)
}

func TestSessionApplyMoveFinishesOnStalemate(t *testing.T) {
	engine := &scriptedEngine{
		fen: "7k/5Q2/7K/8/8/8/8/8 w - - 0 1",
		moves: []scriptedMove{
			{
				input: "f7g7",
				result: MoveResult{
					FEN:  "7k/6Q1/7K/8/8/8/8/8 b - - 1 1",
					Move: "f7g7",
				},
				legalMovesAfter: nil,
			},
		},
	}

	session := NewSession("game-1", GameModeClassic, "user1", "user2", 0, engine)
	session.Status = StatusActive

	state, _, err := session.ApplyMove("user1", "f7g7")
	if err != nil {
		t.Fatalf("apply move: %v", err)
	}
	if state.Status != string(StatusFinished) {
		t.Fatalf("expected finished status, got %q", state.Status)
	}
	if state.FinishedReason != "stalemate" {
		t.Fatalf("expected stalemate reason, got %q", state.FinishedReason)
	}
	if state.WinnerID != "" {
		t.Fatalf("expected no winner, got %q", state.WinnerID)
	}
}

func TestSessionApplyMoveFinishesOnThreefoldRepetition(t *testing.T) {
	engine := NewChessEngine()
	session := NewSession("game-2", GameModeClassic, "user1", "user2", 0, engine)
	session.Status = StatusActive

	moves := []struct {
		userID string
		move   string
	}{
		{userID: "user1", move: "g1f3"},
		{userID: "user2", move: "g8f6"},
		{userID: "user1", move: "f3g1"},
		{userID: "user2", move: "f6g8"},
		{userID: "user1", move: "g1f3"},
		{userID: "user2", move: "g8f6"},
		{userID: "user1", move: "f3g1"},
		{userID: "user2", move: "f6g8"},
	}

	var state State
	var err error
	for _, item := range moves {
		state, _, err = session.ApplyMove(item.userID, item.move)
		if err != nil {
			t.Fatalf("apply move %s: %v", item.move, err)
		}
	}

	if state.Status != string(StatusFinished) {
		t.Fatalf("expected finished status, got %q", state.Status)
	}
	if state.FinishedReason != "threefold_repetition" {
		t.Fatalf("expected threefold repetition, got %q", state.FinishedReason)
	}
	if state.WinnerID != "" {
		t.Fatalf("expected no winner, got %q", state.WinnerID)
	}
}

func TestSessionApplyMoveFinishesOnInsufficientMaterial(t *testing.T) {
	engine := &scriptedEngine{
		fen: "8/8/8/8/8/8/6k1/5BK1 w - - 0 1",
		moves: []scriptedMove{
			{
				input: "f1e2",
				result: MoveResult{
					FEN:  "8/8/8/8/8/8/4B1k1/6K1 b - - 1 1",
					Move: "f1e2",
				},
				legalMovesAfter: []string{"g2h3"},
			},
		},
	}

	session := NewSession("game-3", GameModeClassic, "user1", "user2", 0, engine)
	session.Status = StatusActive

	state, _, err := session.ApplyMove("user1", "f1e2")
	if err != nil {
		t.Fatalf("apply move: %v", err)
	}
	if state.Status != string(StatusFinished) {
		t.Fatalf("expected finished status, got %q", state.Status)
	}
	if state.FinishedReason != "insufficient_material" {
		t.Fatalf("expected insufficient material, got %q", state.FinishedReason)
	}
	if state.WinnerID != "" {
		t.Fatalf("expected no winner, got %q", state.WinnerID)
	}
}
