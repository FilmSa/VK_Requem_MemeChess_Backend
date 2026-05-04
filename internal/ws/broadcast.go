package ws

import (
	"encoding/json"

	"meme_chess/internal/game"
)

func (h *Hub) BroadcastGameState(gameID string, state game.State) {
	payload, err := json.Marshal(OutgoingMessage{
		Type:    "game.state",
		Payload: state,
	})
	if err != nil {
		return
	}

	h.broadcast <- BroadcastMessage{
		GameID:  gameID,
		Payload: payload,
	}
}

func (h *Hub) BroadcastGameFinished(gameID string, state game.State) {
	payload, err := json.Marshal(OutgoingMessage{
		Type: "game.finished",
		Payload: map[string]string{
			"game_id":         gameID,
			"winner_id":       state.WinnerID,
			"finished_reason": state.FinishedReason,
		},
	})
	if err != nil {
		return
	}

	h.broadcast <- BroadcastMessage{
		GameID:  gameID,
		Payload: payload,
	}
}
