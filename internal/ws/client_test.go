package ws

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"meme_chess/internal/game"
)

func TestHandleGameStickerBroadcastsExactAssetURL(t *testing.T) {
	hub := NewHub()
	svc := game.NewService(nil)
	client := &Client{
		hub:         hub,
		gameService: svc,
		send:        make(chan []byte, 4),
		userID:      "player-1",
		gameIDs:     map[string]struct{}{"game-1": {}},
	}

	session, err := svc.CreateGame(context.Background(), "game-1", "player-1", "player-2", game.NewMockEngine())
	if err != nil {
		t.Fatalf("create game: %v", err)
	}
	session.SetConnected("player-1", true)
	session.SetConnected("player-2", true)

	msg := IncomingMessage{
		Type:      "game.sticker",
		RequestID: "req-1",
		Payload: mustRawMessage(t, GameStickerPayload{
			GameID:    "game-1",
			AssetURL:  "https://cdn.example.com/stickers/hype.gif",
			MediaType: "gif",
		}),
	}

	client.handleIncomingMessage(msg)

	accepted := mustOutgoingMessage(t, <-client.send)
	if accepted.Type != "game.sticker.accepted" {
		t.Fatalf("expected acceptance message, got %s", accepted.Type)
	}

	payload, ok := accepted.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected acceptance payload to be an object, got %T", accepted.Payload)
	}
	if got := payload["asset_url"]; got != "https://cdn.example.com/stickers/hype.gif" {
		t.Fatalf("expected exact asset url to be echoed back, got %#v", got)
	}

	broadcastRaw := <-hub.broadcast
	var broadcast OutgoingMessage
	if err := json.Unmarshal(broadcastRaw.Payload, &broadcast); err != nil {
		t.Fatalf("unmarshal broadcast: %v", err)
	}

	if broadcast.Type != "game.sticker" {
		t.Fatalf("expected sticker broadcast, got %s", broadcast.Type)
	}
	if broadcastRaw.GameID != "game-1" {
		t.Fatalf("expected broadcast game id game-1, got %s", broadcastRaw.GameID)
	}

	broadcastPayload, ok := broadcast.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected broadcast payload object, got %T", broadcast.Payload)
	}
	if got := broadcastPayload["asset_url"]; got != "https://cdn.example.com/stickers/hype.gif" {
		t.Fatalf("expected broadcast asset url to match input exactly, got %#v", got)
	}
	if got := broadcastPayload["by_user_id"]; got != "player-1" {
		t.Fatalf("expected broadcast sender player-1, got %#v", got)
	}
}

func TestHandleGameStickerRejectsClientOutsideRoom(t *testing.T) {
	hub := NewHub()
	svc := game.NewService(nil)
	client := &Client{
		hub:         hub,
		gameService: svc,
		send:        make(chan []byte, 1),
		userID:      "player-1",
		gameIDs:     make(map[string]struct{}),
	}

	if _, err := svc.CreateGame(context.Background(), "game-1", "player-1", "player-2", game.NewMockEngine()); err != nil {
		t.Fatalf("create game: %v", err)
	}

	msg := IncomingMessage{
		Type:      "game.sticker",
		RequestID: "req-2",
		Payload: mustRawMessage(t, GameStickerPayload{
			GameID:   "game-1",
			AssetURL: "https://cdn.example.com/stickers/hype.gif",
		}),
	}

	client.handleIncomingMessage(msg)

	reply := mustOutgoingMessage(t, <-client.send)
	if reply.Type != "error" {
		t.Fatalf("expected error reply, got %s", reply.Type)
	}
	if reply.Error == nil || reply.Error.Code != "GAME_NOT_JOINED" {
		t.Fatalf("expected GAME_NOT_JOINED, got %+v", reply.Error)
	}
}

func TestBroadcastMoveOutcomeIncludesEvolutionEffects(t *testing.T) {
	hub := NewHub()
	client := &Client{hub: hub}

	client.broadcastMoveOutcome("game-1", "player-1", game.State{}, game.MoveResult{
		Move: "a1a4",
		Effects: []game.MoveEffect{
			{
				Type:  game.EffectTypeRookRampage,
				Title: "Rook rampage",
			},
		},
	})

	broadcastRaw := <-hub.broadcast
	var broadcast OutgoingMessage
	if err := json.Unmarshal(broadcastRaw.Payload, &broadcast); err != nil {
		t.Fatalf("unmarshal broadcast: %v", err)
	}

	if broadcast.Type != "game.event.evolution" {
		t.Fatalf("expected evolution event, got %s", broadcast.Type)
	}

	payload, ok := broadcast.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected payload object, got %T", broadcast.Payload)
	}
	if got := payload["move"]; got != "a1a4" {
		t.Fatalf("expected move a1a4, got %#v", got)
	}

	effects, ok := payload["effects"].([]any)
	if !ok || len(effects) != 1 {
		t.Fatalf("expected 1 effect in payload, got %#v", payload["effects"])
	}
}

func TestHandleGameMoveDelaysBotResponse(t *testing.T) {
	hub := NewHub()
	svc := game.NewService(nil)

	gameID, err := svc.CreateBotGame(context.Background(), "player-1", game.GameModeClassic, "medium")
	if err != nil {
		t.Fatalf("create bot game: %v", err)
	}
	if _, err := svc.JoinGame(context.Background(), gameID, "player-1"); err != nil {
		t.Fatalf("join bot game: %v", err)
	}

	var (
		scheduledDelay time.Duration
		scheduledFn    func()
	)

	client := &Client{
		hub:         hub,
		gameService: svc,
		send:        make(chan []byte, 4),
		userID:      "player-1",
		gameIDs:     map[string]struct{}{gameID: {}},
		botDelay:    botMoveDelay,
		afterFunc: func(delay time.Duration, fn func()) *time.Timer {
			scheduledDelay = delay
			scheduledFn = fn
			return nil
		},
	}

	client.handleIncomingMessage(IncomingMessage{
		Type:      "game.move",
		RequestID: "req-bot-delay",
		Payload: mustRawMessage(t, GameMovePayload{
			GameID: gameID,
			Move:   "e2e4",
		}),
	})

	accepted := mustOutgoingMessage(t, <-client.send)
	if accepted.Type != "game.move.accepted" {
		t.Fatalf("expected move acceptance, got %s", accepted.Type)
	}

	if scheduledDelay != 4*time.Second {
		t.Fatalf("expected bot move to be delayed by 4s, got %s", scheduledDelay)
	}
	if scheduledFn == nil {
		t.Fatal("expected bot move callback to be scheduled")
	}

	firstBroadcast := mustOutgoingMessage(t, (<-hub.broadcast).Payload)
	if firstBroadcast.Type != "game.state" {
		t.Fatalf("expected first broadcast to be game.state, got %s", firstBroadcast.Type)
	}

	select {
	case msg := <-hub.broadcast:
		unexpected := mustOutgoingMessage(t, msg.Payload)
		t.Fatalf("expected no bot broadcast before delayed callback, got %s", unexpected.Type)
	default:
	}

	scheduledFn()

	secondBroadcast := mustOutgoingMessage(t, (<-hub.broadcast).Payload)
	if secondBroadcast.Type != "game.state" {
		t.Fatalf("expected delayed bot response to broadcast game.state, got %s", secondBroadcast.Type)
	}

	payload, ok := secondBroadcast.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected state payload object, got %T", secondBroadcast.Payload)
	}
	moves, ok := payload["moves"].([]any)
	if !ok || len(moves) != 2 {
		t.Fatalf("expected two moves after delayed bot turn, got %#v", payload["moves"])
	}
}

func TestHandleJoinGameSchedulesBotResponseWhenBotTurnPending(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	svc := game.NewService(nil)

	gameID, err := svc.CreateBotGame(context.Background(), "player-1", game.GameModeClassic, "medium")
	if err != nil {
		t.Fatalf("create bot game: %v", err)
	}
	if _, err := svc.JoinGame(context.Background(), gameID, "player-1"); err != nil {
		t.Fatalf("initial join bot game: %v", err)
	}
	state, _, err := svc.MakeMove(context.Background(), gameID, "player-1", "e2e4")
	if err != nil {
		t.Fatalf("make human move: %v", err)
	}
	if state.CurrentTurnUserID != state.Player2ID {
		t.Fatalf("expected bot turn to be pending, got current_turn_user_id=%q player2_id=%q", state.CurrentTurnUserID, state.Player2ID)
	}

	var (
		scheduledDelay time.Duration
		scheduledFn    func()
	)

	client := &Client{
		hub:         hub,
		gameService: svc,
		send:        make(chan []byte, 8),
		userID:      "player-1",
		gameIDs:     make(map[string]struct{}),
		botDelay:    botMoveDelay,
		afterFunc: func(delay time.Duration, fn func()) *time.Timer {
			scheduledDelay = delay
			scheduledFn = fn
			return nil
		},
	}

	client.handleJoinGame(IncomingMessage{
		Type:      "game.join",
		RequestID: "req-bot-rejoin",
		Payload: mustRawMessage(t, JoinGamePayload{
			GameID: gameID,
		}),
	})

	joined := mustOutgoingMessage(t, <-client.send)
	if joined.Type != "game.joined" {
		t.Fatalf("expected join reply, got %s", joined.Type)
	}

	if scheduledDelay != 4*time.Second {
		t.Fatalf("expected bot move to be delayed by 4s after rejoin, got %s", scheduledDelay)
	}
	if scheduledFn == nil {
		t.Fatal("expected bot move callback to be scheduled after rejoin")
	}
}

func mustRawMessage(t *testing.T, v any) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return data
}

func mustOutgoingMessage(t *testing.T, raw []byte) OutgoingMessage {
	t.Helper()

	var msg OutgoingMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal outgoing message: %v", err)
	}
	return msg
}
