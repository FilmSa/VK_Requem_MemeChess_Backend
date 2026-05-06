package game

import (
	"context"
	"testing"

	"meme_chess/internal/analyzer/analysis"
)

type stubMoveAnalyzer struct {
	started  []string
	recorded []string
	result   *analysis.Result
}

func (s *stubMoveAnalyzer) StartGame(gameID string) {
	s.started = append(s.started, gameID)
}

func (s *stubMoveAnalyzer) RecordMove(gameID, move string) {
	s.recorded = append(s.recorded, gameID+":"+move)
}

func (s *stubMoveAnalyzer) AnalyzeRecordedMove(gameID, move string, moveNumber int, depth int) (*analysis.Result, error) {
	return s.result, nil
}

func (s *stubMoveAnalyzer) ForgetGame(gameID string) {}

func (s *stubMoveAnalyzer) SyncGame(gameID string, moves []string) error { return nil }

func TestService_StartsAndRecordsWithMoveAnalyzer(t *testing.T) {
	svc := NewService(nil)
	stub := &stubMoveAnalyzer{
		result: &analysis.Result{Move: "e2e4", Quality: "best", Depth: 3},
	}
	svc.SetMoveAnalyzer(stub)

	_, err := svc.CreateGame(context.Background(), "game-123", "user1", "user2", NewChessEngine())
	if err != nil {
		t.Fatalf("create game: %v", err)
	}
	if len(stub.started) != 1 || stub.started[0] != "game-123" {
		t.Fatalf("expected analyzer to start tracking game-123, got %+v", stub.started)
	}

	activateGame(t, svc)
	if _, _, err := svc.MakeMove(context.Background(), "game-123", "user1", "e2e4"); err != nil {
		t.Fatalf("make move: %v", err)
	}
	if len(stub.recorded) != 1 || stub.recorded[0] != "game-123:e2e4" {
		t.Fatalf("expected analyzer to record move, got %+v", stub.recorded)
	}
}

func TestService_AnalyzeMoveUsesMoveAnalyzer(t *testing.T) {
	svc := newTestServiceWithGame()
	stub := &stubMoveAnalyzer{
		result: &analysis.Result{Move: "e2e4", Quality: "best", Depth: 3},
	}
	svc.SetMoveAnalyzer(stub)

	activateGame(t, svc)
	if _, _, err := svc.MakeMove(context.Background(), "game-123", "user1", "e2e4"); err != nil {
		t.Fatalf("make move: %v", err)
	}

	result, err := svc.AnalyzeMove("game-123", "user1", "e2e4", 1, 3)
	if err != nil {
		t.Fatalf("analyze move: %v", err)
	}
	if result == nil || result.Move != "e2e4" {
		t.Fatalf("expected analyzer result for e2e4, got %+v", result)
	}
}
