package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"meme_chess/internal/game"
	"meme_chess/internal/inventory"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 8192
)

type Client struct {
	hub         *Hub
	gameService *game.Service
	invService  *inventory.Service
	conn        *websocket.Conn
	send        chan []byte
	userID      string
	gameIDs     map[string]struct{}
}

func NewClient(hub *Hub, gameService *game.Service, invService *inventory.Service, conn *websocket.Conn, userID string) *Client {
	return &Client{
		hub:         hub,
		gameService: gameService,
		invService:  invService,
		conn:        conn,
		send:        make(chan []byte, 256),
		userID:      userID,
		gameIDs:     make(map[string]struct{}),
	}
}

func (c *Client) readPump() {
	defer func() {
		for gameID := range c.gameIDs {
			_ = c.gameService.LeaveGame(gameID, c.userID)
			c.broadcastGameState(gameID)
		}

		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var msg IncomingMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			c.sendError("", "BAD_REQUEST", "invalid message format")
			continue
		}

		c.handleIncomingMessage(msg)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleIncomingMessage(msg IncomingMessage) {
	switch msg.Type {
	case "game.join":
		c.handleJoinGame(msg)

	case "game.move":
		c.handleGameMove(msg)

	case "game.resign":
		c.handleGameResign(msg)

	case "game.draw.offer":
		c.handleGameDrawOffer(msg)

	case "game.draw.accept":
		c.handleGameDrawAccept(msg)

	case "game.draw.decline":
		c.handleGameDrawDecline(msg)

	case "game.emote":
		c.handleGameEmote(msg)

	case "game.sticker":
		c.handleGameSticker(msg)

	default:
		c.sendError(msg.RequestID, "UNKNOWN_TYPE", "unknown message type")
	}
}

func (c *Client) handleGameResign(msg IncomingMessage) {
	var payload GameResignPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendError(msg.RequestID, "BAD_REQUEST", "invalid resign payload")
		return
	}
	if strings.TrimSpace(payload.GameID) == "" {
		c.sendError(msg.RequestID, "BAD_REQUEST", "game_id is required")
		return
	}
	if _, ok := c.gameIDs[payload.GameID]; !ok {
		c.sendError(msg.RequestID, "GAME_NOT_JOINED", "join the game room before resigning")
		return
	}

	state, err := c.gameService.Resign(context.Background(), payload.GameID, c.userID)
	if err != nil {
		c.sendGameError(msg.RequestID, err)
		return
	}

	c.sendJSON(OutgoingMessage{
		Type:      "game.resign.accepted",
		RequestID: msg.RequestID,
		Payload: map[string]string{
			"game_id": payload.GameID,
		},
	})

	c.broadcastJSON(payload.GameID, OutgoingMessage{
		Type:    "game.state",
		Payload: state,
	})

	c.broadcastJSON(payload.GameID, OutgoingMessage{
		Type:    "game.finished",
		Payload: buildFinishedPayload(payload.GameID, state),
	})
}

func (c *Client) handleGameDrawOffer(msg IncomingMessage) {
	var payload GameDrawPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendError(msg.RequestID, "BAD_REQUEST", "invalid draw offer payload")
		return
	}
	payload.GameID = strings.TrimSpace(payload.GameID)
	if payload.GameID == "" {
		c.sendError(msg.RequestID, "BAD_REQUEST", "game_id is required")
		return
	}
	if _, ok := c.gameIDs[payload.GameID]; !ok {
		c.sendError(msg.RequestID, "GAME_NOT_JOINED", "join the game room before offering a draw")
		return
	}

	state, err := c.gameService.OfferDraw(context.Background(), payload.GameID, c.userID)
	if err != nil {
		c.sendGameError(msg.RequestID, err)
		return
	}

	c.sendJSON(OutgoingMessage{
		Type:      "game.draw.offer.accepted",
		RequestID: msg.RequestID,
		Payload: map[string]string{
			"game_id": payload.GameID,
		},
	})

	c.broadcastJSON(payload.GameID, OutgoingMessage{
		Type:    "game.state",
		Payload: state,
	})

	c.broadcastJSON(payload.GameID, OutgoingMessage{
		Type: "game.event.draw_offered",
		Payload: map[string]string{
			"game_id":       payload.GameID,
			"by_user_id":    c.userID,
			"offered_by_id": c.userID,
		},
	})
}

func (c *Client) handleGameDrawDecline(msg IncomingMessage) {
	var payload GameDrawPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendError(msg.RequestID, "BAD_REQUEST", "invalid draw decline payload")
		return
	}
	payload.GameID = strings.TrimSpace(payload.GameID)
	if payload.GameID == "" {
		c.sendError(msg.RequestID, "BAD_REQUEST", "game_id is required")
		return
	}
	if _, ok := c.gameIDs[payload.GameID]; !ok {
		c.sendError(msg.RequestID, "GAME_NOT_JOINED", "join the game room before declining a draw")
		return
	}

	state, err := c.gameService.DeclineDraw(context.Background(), payload.GameID, c.userID)
	if err != nil {
		c.sendGameError(msg.RequestID, err)
		return
	}

	c.sendJSON(OutgoingMessage{
		Type:      "game.draw.decline.accepted",
		RequestID: msg.RequestID,
		Payload: map[string]string{
			"game_id": payload.GameID,
		},
	})

	c.broadcastJSON(payload.GameID, OutgoingMessage{
		Type:    "game.state",
		Payload: state,
	})

	c.broadcastJSON(payload.GameID, OutgoingMessage{
		Type: "game.event.draw_declined",
		Payload: map[string]string{
			"game_id":    payload.GameID,
			"by_user_id": c.userID,
		},
	})
}

func (c *Client) handleGameDrawAccept(msg IncomingMessage) {
	var payload GameDrawPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendError(msg.RequestID, "BAD_REQUEST", "invalid draw accept payload")
		return
	}
	payload.GameID = strings.TrimSpace(payload.GameID)
	if payload.GameID == "" {
		c.sendError(msg.RequestID, "BAD_REQUEST", "game_id is required")
		return
	}
	if _, ok := c.gameIDs[payload.GameID]; !ok {
		c.sendError(msg.RequestID, "GAME_NOT_JOINED", "join the game room before accepting a draw")
		return
	}

	state, err := c.gameService.AcceptDraw(context.Background(), payload.GameID, c.userID)
	if err != nil {
		c.sendGameError(msg.RequestID, err)
		return
	}

	c.sendJSON(OutgoingMessage{
		Type:      "game.draw.accept.accepted",
		RequestID: msg.RequestID,
		Payload: map[string]string{
			"game_id": payload.GameID,
		},
	})

	c.broadcastJSON(payload.GameID, OutgoingMessage{
		Type:    "game.state",
		Payload: state,
	})

	c.broadcastJSON(payload.GameID, OutgoingMessage{
		Type:    "game.finished",
		Payload: buildFinishedPayload(payload.GameID, state),
	})
}

func (c *Client) handleJoinGame(msg IncomingMessage) {
	var payload JoinGamePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendError(msg.RequestID, "BAD_REQUEST", "invalid join payload")
		return
	}

	if payload.GameID == "" {
		c.sendError(msg.RequestID, "BAD_REQUEST", "game_id is required")
		return
	}

	state, err := c.gameService.JoinGame(context.Background(), payload.GameID, c.userID)
	if err != nil {
		c.sendGameError(msg.RequestID, err)
		return
	}

	c.hub.joinRoom <- subscription{
		client: c,
		gameID: payload.GameID,
	}

	c.sendJSON(OutgoingMessage{
		Type:      "game.joined",
		RequestID: msg.RequestID,
		Payload:   state,
	})

	c.broadcastGameState(payload.GameID)
}

func (c *Client) handleGameMove(msg IncomingMessage) {
	var payload GameMovePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendError(msg.RequestID, "BAD_REQUEST", "invalid move payload")
		return
	}

	if payload.GameID == "" {
		c.sendError(msg.RequestID, "BAD_REQUEST", "game_id is required")
		return
	}
	if payload.Move == "" {
		c.sendError(msg.RequestID, "BAD_REQUEST", "move is required")
		return
	}

	state, result, err := c.gameService.MakeMove(context.Background(), payload.GameID, c.userID, payload.Move)
	if err != nil {
		if errors.Is(err, game.ErrTimeExpired) {
			c.broadcastJSON(payload.GameID, OutgoingMessage{
				Type:    "game.state",
				Payload: state,
			})
			c.broadcastJSON(payload.GameID, OutgoingMessage{
				Type:    "game.finished",
				Payload: buildFinishedPayload(payload.GameID, state),
			})
		}
		c.sendGameError(msg.RequestID, err)
		return
	}

	c.sendJSON(OutgoingMessage{
		Type:      "game.move.accepted",
		RequestID: msg.RequestID,
		Payload: map[string]string{
			"game_id": payload.GameID,
			"move":    payload.Move,
		},
	})

	c.broadcastJSON(payload.GameID, OutgoingMessage{
		Type:    "game.state",
		Payload: state,
	})

	c.broadcastMoveOutcome(payload.GameID, c.userID, state, result)

	botState, botResult, moved, err := c.gameService.PlayBotTurn(context.Background(), payload.GameID)
	if err != nil {
		c.sendError("", "BOT_MOVE_FAILED", "failed to compute bot move")
		return
	}
	if moved {
		c.broadcastJSON(payload.GameID, OutgoingMessage{
			Type:    "game.state",
			Payload: botState,
		})
		c.broadcastMoveOutcome(payload.GameID, botState.Player2ID, botState, botResult)
		return
	}

	if botState.Status == string(game.StatusFinished) && botState.FinishedReason == "stalemate" {
		c.broadcastJSON(payload.GameID, OutgoingMessage{
			Type:    "game.state",
			Payload: botState,
		})
		c.broadcastJSON(payload.GameID, OutgoingMessage{
			Type:    "game.finished",
			Payload: buildFinishedPayload(payload.GameID, botState),
		})
	}
}

func (c *Client) handleGameSticker(msg IncomingMessage) {
	var payload GameStickerPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendError(msg.RequestID, "BAD_REQUEST", "invalid sticker payload")
		return
	}

	payload.GameID = strings.TrimSpace(payload.GameID)
	payload.StickerSlug = strings.TrimSpace(payload.StickerSlug)
	payload.StickerID = strings.TrimSpace(payload.StickerID)
	payload.Title = strings.TrimSpace(payload.Title)
	payload.AssetURL = strings.TrimSpace(payload.AssetURL)
	payload.MediaType = strings.TrimSpace(payload.MediaType)
	payload.ImageURL = strings.TrimSpace(payload.ImageURL)
	payload.VideoURL = strings.TrimSpace(payload.VideoURL)
	payload.SoundURL = strings.TrimSpace(payload.SoundURL)
	if payload.GameID == "" {
		c.sendError(msg.RequestID, "BAD_REQUEST", "game_id is required")
		return
	}
	if _, ok := c.gameIDs[payload.GameID]; !ok {
		c.sendError(msg.RequestID, "GAME_NOT_JOINED", "join the game room before sending stickers")
		return
	}

	session, ok := c.gameService.GetSession(payload.GameID)
	if !ok {
		c.sendError(msg.RequestID, "GAME_NOT_FOUND", "game not found")
		return
	}
	if !session.HasPlayer(c.userID) {
		c.sendError(msg.RequestID, "FORBIDDEN", "you are not a participant of this game")
		return
	}

	if payload.StickerSlug != "" {
		if c.invService == nil {
			c.sendError(msg.RequestID, "INVENTORY_UNAVAILABLE", "inventory service unavailable")
			return
		}
		assetURL, err := c.invService.ResolveSelectedStickerAssetURL(context.Background(), c.userID, payload.StickerSlug)
		if err != nil {
			switch {
			case errors.Is(err, inventory.ErrStickerNotSelected):
				c.sendError(msg.RequestID, "STICKER_NOT_SELECTED", "sticker is not selected")
			case errors.Is(err, inventory.ErrItemNotOwned):
				c.sendError(msg.RequestID, "STICKER_NOT_OWNED", "sticker is not owned")
			case errors.Is(err, inventory.ErrItemNotFound):
				c.sendError(msg.RequestID, "STICKER_NOT_FOUND", "sticker not found")
			default:
				c.sendError(msg.RequestID, "INTERNAL_ERROR", "failed to resolve sticker")
			}
			return
		}
		payload.AssetURL = strings.TrimSpace(assetURL)
	}

	if payload.AssetURL == "" {
		c.sendError(msg.RequestID, "BAD_REQUEST", "asset_url or sticker_slug is required")
		return
	}

	c.sendJSON(OutgoingMessage{
		Type:      "game.sticker.accepted",
		RequestID: msg.RequestID,
		Payload: map[string]string{
			"game_id":      payload.GameID,
			"sticker_slug": payload.StickerSlug,
			"sticker_id":   payload.StickerID,
			"title":        payload.Title,
			"asset_url":    payload.AssetURL,
			"media_type":   payload.MediaType,
			"image_url":    payload.ImageURL,
			"video_url":    payload.VideoURL,
			"sound_url":    payload.SoundURL,
		},
	})

	c.broadcastJSON(payload.GameID, OutgoingMessage{
		Type: "game.sticker",
		Payload: map[string]string{
			"game_id":      payload.GameID,
			"by_user_id":   c.userID,
			"sticker_slug": payload.StickerSlug,
			"sticker_id":   payload.StickerID,
			"title":        payload.Title,
			"asset_url":    payload.AssetURL,
			"media_type":   payload.MediaType,
			"image_url":    payload.ImageURL,
			"video_url":    payload.VideoURL,
			"sound_url":    payload.SoundURL,
		},
	})
}

func (c *Client) handleGameEmote(msg IncomingMessage) {
	var payload GameEmotePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendError(msg.RequestID, "BAD_REQUEST", "invalid emote payload")
		return
	}

	if payload.GameID == "" {
		c.sendError(msg.RequestID, "BAD_REQUEST", "game_id is required")
		return
	}

	emoteSlug := strings.TrimSpace(payload.EmoteSlug)
	emoteMP4 := strings.TrimSpace(payload.EmoteMP4)
	stickerPayload := GameStickerPayload{
		GameID:      strings.TrimSpace(payload.GameID),
		StickerSlug: emoteSlug,
		AssetURL:    emoteMP4,
		MediaType:   "",
		ImageURL:    "",
		VideoURL:    "",
		SoundURL:    "",
	}
	raw, _ := json.Marshal(stickerPayload)
	c.handleGameSticker(IncomingMessage{
		Type:      "game.sticker",
		RequestID: msg.RequestID,
		Payload:   raw,
	})
}

func (c *Client) broadcastGameState(gameID string) {
	session, ok := c.gameService.GetSession(gameID)
	if !ok {
		return
	}

	c.broadcastJSON(gameID, OutgoingMessage{
		Type:    "game.state",
		Payload: session.Snapshot(),
	})
}

func (c *Client) broadcastMoveOutcome(gameID string, actorUserID string, state game.State, result game.MoveResult) {
	if len(result.Effects) > 0 {
		c.broadcastJSON(gameID, OutgoingMessage{
			Type: "game.event.evolution",
			Payload: map[string]any{
				"game_id":    gameID,
				"by_user_id": actorUserID,
				"move":       result.Move,
				"effects":    result.Effects,
			},
		})
	}

	if result.IsCapture {
		c.broadcastJSON(gameID, OutgoingMessage{
			Type: "game.event.capture",
			Payload: map[string]string{
				"game_id":    gameID,
				"by_user_id": actorUserID,
				"move":       result.Move,
			},
		})
	}

	if result.IsCheck {
		c.broadcastJSON(gameID, OutgoingMessage{
			Type: "game.event.check",
			Payload: map[string]string{
				"game_id":    gameID,
				"by_user_id": actorUserID,
				"move":       result.Move,
			},
		})
	}

	if result.IsCheckmate {
		c.broadcastJSON(gameID, OutgoingMessage{
			Type: "game.event.checkmate",
			Payload: map[string]string{
				"game_id":    gameID,
				"by_user_id": actorUserID,
				"move":       result.Move,
			},
		})

		c.broadcastJSON(gameID, OutgoingMessage{
			Type:    "game.finished",
			Payload: buildFinishedPayload(gameID, state),
		})
	}
}

func (c *Client) broadcastJSON(gameID string, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Println("marshal error:", err)
		return
	}

	c.hub.broadcast <- BroadcastMessage{
		GameID:  gameID,
		Payload: data,
	}
}

func (c *Client) sendGameError(requestID string, err error) {
	switch {
	case errors.Is(err, game.ErrGameNotFound):
		c.sendError(requestID, "GAME_NOT_FOUND", "game not found")
	case errors.Is(err, game.ErrForbidden):
		c.sendError(requestID, "FORBIDDEN", "you are not a participant of this game")
	case errors.Is(err, game.ErrGameFull):
		c.sendError(requestID, "GAME_FULL", "game already has two players")
	case errors.Is(err, game.ErrInviteExpired):
		c.sendError(requestID, "INVITE_EXPIRED", "invite token expired")
	case errors.Is(err, game.ErrNotYourTurn):
		c.sendError(requestID, "NOT_YOUR_TURN", "it is not your turn")
	case errors.Is(err, game.ErrGameFinished):
		c.sendError(requestID, "GAME_FINISHED", "game already finished")
	case errors.Is(err, game.ErrGameNotActive):
		c.sendError(requestID, "GAME_NOT_ACTIVE", "game is not active yet")
	case errors.Is(err, game.ErrInvalidMove):
		c.sendError(requestID, "INVALID_MOVE", "invalid move")
	case errors.Is(err, game.ErrTimeExpired):
		c.sendError(requestID, "TIME_EXPIRED", "your time has expired")
	default:
		c.sendError(requestID, "INTERNAL_ERROR", "internal error")
	}
}

func (c *Client) sendError(requestID, code, message string) {
	c.sendJSON(OutgoingMessage{
		Type:      "error",
		RequestID: requestID,
		Error: &ErrorBody{
			Code:    code,
			Message: message,
		},
	})
}

func (c *Client) sendJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}

	select {
	case c.send <- data:
	default:
	}
}
