package game

import (
	"context"
	"testing"

	"meme_chess/internal/analyzer/position"
	"meme_chess/internal/analyzer/rules"
)

func TestCreateBotGameStartsActiveSession(t *testing.T) {
	svc := NewService(nil)

	gameID, err := svc.CreateBotGame(context.Background(), "user1", GameModeClassic, botEasy)
	if err != nil {
		t.Fatalf("create bot game: %v", err)
	}

	state, err := svc.JoinGame(context.Background(), gameID, "user1")
	if err != nil {
		t.Fatalf("join bot game: %v", err)
	}

	if !state.BotGame {
		t.Fatal("expected bot game flag to be set")
	}
	if state.BotDifficulty != botEasy {
		t.Fatalf("expected bot difficulty %q, got %q", botEasy, state.BotDifficulty)
	}
	if state.Player2ID != botUserID {
		t.Fatalf("expected bot player id %q, got %q", botUserID, state.Player2ID)
	}
	if state.Status != string(StatusActive) {
		t.Fatalf("expected active status, got %q", state.Status)
	}
}

func TestBotRespondsAfterHumanMove(t *testing.T) {
	svc := NewService(nil)

	gameID, err := svc.CreateBotGame(context.Background(), "user1", GameModeClassic, botMedium)
	if err != nil {
		t.Fatalf("create bot game: %v", err)
	}

	if _, err := svc.JoinGame(context.Background(), gameID, "user1"); err != nil {
		t.Fatalf("join bot game: %v", err)
	}

	state, result, err := svc.MakeMove(context.Background(), gameID, "user1", "e2e4")
	if err != nil {
		t.Fatalf("make human move: %v", err)
	}
	if result.Move != "e2e4" {
		t.Fatalf("expected human move e2e4, got %q", result.Move)
	}
	if state.CurrentTurnUserID != botUserID {
		t.Fatalf("expected bot turn after human move, got %q", state.CurrentTurnUserID)
	}

	botState, botResult, moved, err := svc.PlayBotTurn(context.Background(), gameID)
	if err != nil {
		t.Fatalf("play bot turn: %v", err)
	}
	if !moved {
		t.Fatal("expected bot to make a move")
	}
	if botResult.Move == "" {
		t.Fatal("expected bot move to be recorded")
	}
	if botState.CurrentTurnUserID != "user1" {
		t.Fatalf("expected turn to return to human, got %q", botState.CurrentTurnUserID)
	}
	if len(botState.Moves) != 2 {
		t.Fatalf("expected 2 moves in history after bot response, got %d", len(botState.Moves))
	}
}

func TestHardBotFindsMateInOne(t *testing.T) {
	state := position.NewInitial()
	for _, raw := range []string{"f2f3", "e7e5", "g2g4"} {
		move, err := position.ParseUCIMove(state, raw)
		if err != nil {
			t.Fatalf("parse move %s: %v", raw, err)
		}
		if err := state.ApplyMove(move); err != nil {
			t.Fatalf("apply move %s: %v", raw, err)
		}
	}

	move, err := chooseAnalyzerMove(&analyzerRuntime{
		state: state,
		rules: rules.NewClassicalRuleSet(),
	}, botHardDepth)
	if err != nil {
		t.Fatalf("choose analyzer move: %v", err)
	}

	if move != "d8h4" {
		t.Fatalf("expected mate in one d8h4, got %q", move)
	}
}
