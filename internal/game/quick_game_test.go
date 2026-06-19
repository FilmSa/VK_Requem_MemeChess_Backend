package game

import (
	"context"
	"testing"
)

func TestSearchQuickGame_InvalidRange(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.SearchQuickGame(context.Background(), QuickGameSearchInput{
		UserID:   "u1",
		MinStake: 20,
		MaxStake: 10,
	})
	if err != ErrInvalidStakeRange {
		t.Fatalf("expected ErrInvalidStakeRange, got %v", err)
	}
}

func TestSearchQuickGame_CreatesWaitingRoomWhenNoMatch(t *testing.T) {
	svc := NewService(nil)

	result, err := svc.SearchQuickGame(context.Background(), QuickGameSearchInput{
		UserID:   "u1",
		MinStake: 10,
		MaxStake: 50,
	})
	if err != nil {
		t.Fatalf("search quick game failed: %v", err)
	}
	if result.Status != "waiting" {
		t.Fatalf("expected waiting status, got %q", result.Status)
	}
	if result.GameID == "" {
		t.Fatal("expected game_id")
	}
	if result.AgreedStake < 10 || result.AgreedStake > 50 {
		t.Fatalf("expected stake in range, got %d", result.AgreedStake)
	}
	if !isQuickGameMode(result.GameMode) {
		t.Fatalf("expected supported quick game mode, got %q", result.GameMode)
	}
	if result.TimeControlID == "" || result.TimeControlID == TimeControlUnlimited {
		t.Fatalf("expected timed control, got %q", result.TimeControlID)
	}

	session, ok := svc.GetSession(result.GameID)
	if !ok {
		t.Fatal("expected quick game session")
	}
	if !session.QuickGame {
		t.Fatal("expected quick game flag")
	}
}

func TestSearchQuickGame_JoinsExistingRoomRegardlessOfModeAndTime(t *testing.T) {
	svc := NewService(nil)

	first, err := svc.SearchQuickGame(context.Background(), QuickGameSearchInput{
		UserID:   "host",
		MinStake: 20,
		MaxStake: 20,
	})
	if err != nil {
		t.Fatalf("create quick room: %v", err)
	}

	result, err := svc.SearchQuickGame(context.Background(), QuickGameSearchInput{
		UserID:   "guest",
		MinStake: 10,
		MaxStake: 30,
	})
	if err != nil {
		t.Fatalf("join quick room: %v", err)
	}
	if result.Status != "joined" {
		t.Fatalf("expected joined status, got %q", result.Status)
	}
	if result.GameID != first.GameID {
		t.Fatalf("expected to join %q, got %q", first.GameID, result.GameID)
	}
	if result.AgreedStake != first.AgreedStake {
		t.Fatalf("expected stake %d, got %d", first.AgreedStake, result.AgreedStake)
	}
	if result.GameMode != first.GameMode {
		t.Fatalf("expected mode %q, got %q", first.GameMode, result.GameMode)
	}
	if result.TimeControlID != first.TimeControlID {
		t.Fatalf("expected time control %q, got %q", first.TimeControlID, result.TimeControlID)
	}

	state, err := svc.ReserveInviteSeat(context.Background(), first.GameID, "guest")
	if err != nil {
		t.Fatalf("guest should already be seated: %v", err)
	}
	if state.Player2ID != "guest" {
		t.Fatalf("expected guest as player2, got %q", state.Player2ID)
	}
}

func TestSearchQuickGame_DoesNotJoinOwnRoom(t *testing.T) {
	svc := NewService(nil)

	first, err := svc.SearchQuickGame(context.Background(), QuickGameSearchInput{
		UserID:   "host",
		MinStake: 10,
		MaxStake: 10,
	})
	if err != nil {
		t.Fatalf("create quick room: %v", err)
	}

	second, err := svc.SearchQuickGame(context.Background(), QuickGameSearchInput{
		UserID:   "host",
		MinStake: 10,
		MaxStake: 10,
	})
	if err != nil {
		t.Fatalf("repeat quick search: %v", err)
	}
	if second.Status != "waiting" {
		t.Fatalf("expected waiting status, got %q", second.Status)
	}
	if second.GameID != first.GameID {
		t.Fatalf("expected same room %q, got %q", first.GameID, second.GameID)
	}
}

func TestSearchQuickGame_SkipsNonOverlappingStake(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.SearchQuickGame(context.Background(), QuickGameSearchInput{
		UserID:   "host",
		MinStake: 100,
		MaxStake: 100,
	})
	if err != nil {
		t.Fatalf("create quick room: %v", err)
	}

	result, err := svc.SearchQuickGame(context.Background(), QuickGameSearchInput{
		UserID:   "guest",
		MinStake: 10,
		MaxStake: 50,
	})
	if err != nil {
		t.Fatalf("search quick game: %v", err)
	}
	if result.Status != "waiting" {
		t.Fatalf("expected new waiting room, got %q", result.Status)
	}
	if result.GameID == "" {
		t.Fatal("expected new game_id")
	}
}

func TestLeaveQuickGameSearch_CancelsWaitingRoom(t *testing.T) {
	svc := NewService(nil)

	result, err := svc.SearchQuickGame(context.Background(), QuickGameSearchInput{
		UserID:   "host",
		MinStake: 10,
		MaxStake: 10,
	})
	if err != nil {
		t.Fatalf("create quick room: %v", err)
	}

	leaveResult, err := svc.LeaveQuickGameSearch(context.Background(), "host")
	if err != nil {
		t.Fatalf("leave quick game: %v", err)
	}
	if leaveResult.Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %q", leaveResult.Status)
	}

	if _, ok := svc.GetSession(result.GameID); ok {
		t.Fatal("expected quick room to be removed")
	}
}

func TestLeaveQuickGameSearch_IdleWhenNotWaiting(t *testing.T) {
	svc := NewService(nil)

	result, err := svc.LeaveQuickGameSearch(context.Background(), "missing")
	if err != nil {
		t.Fatalf("leave quick game: %v", err)
	}
	if result.Status != "idle" {
		t.Fatalf("expected idle status, got %q", result.Status)
	}
}
