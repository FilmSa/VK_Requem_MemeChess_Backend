package ws

import (
	"encoding/json"
	"time"

	"meme_chess/internal/game"
)

func buildFinishedPayload(gameID string, state game.State) map[string]any {
	return map[string]any{
		"game_id":                   gameID,
		"winner_id":                 state.WinnerID,
		"finished_reason":           state.FinishedReason,
		"time_control_id":           state.TimeControlID,
		"time_control_label":        state.TimeControlLabel,
		"time_control_base_ms":      state.TimeControlBaseMs,
		"time_control_increment_ms": state.TimeControlIncrementMs,
		"player1_remaining_ms":      state.Player1RemainingMs,
		"player2_remaining_ms":      state.Player2RemainingMs,
		"current_turn_started_at": func() string {
			if state.CurrentTurnStartedAt.IsZero() {
				return ""
			}
			return state.CurrentTurnStartedAt.UTC().Format(time.RFC3339Nano)
		}(),
	}
}

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
		Type:    "game.finished",
		Payload: buildFinishedPayload(gameID, state),
	})
	if err != nil {
		return
	}

	h.broadcast <- BroadcastMessage{
		GameID:  gameID,
		Payload: payload,
	}
}
