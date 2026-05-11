package game

import (
	"encoding/json"
	"time"
)

type timeControlPayload struct {
	TimeControlID          string `json:"time_control_id"`
	TimeControlLabel       string `json:"time_control_label"`
	TimeControlBaseMs      int64  `json:"time_control_base_ms"`
	TimeControlIncrementMs int64  `json:"time_control_increment_ms"`
	Player1RemainingMs     int64  `json:"player1_remaining_ms"`
	Player2RemainingMs     int64  `json:"player2_remaining_ms"`
	CurrentTurnStartedAt   string `json:"current_turn_started_at"`
}

func buildTimeControlPayload(
	timeControlID string,
	timeControlLabel string,
	timeControlBaseMs int64,
	timeControlIncrementMs int64,
	player1RemainingMs int64,
	player2RemainingMs int64,
	currentTurnStartedAt time.Time,
) timeControlPayload {
	return timeControlPayload{
		TimeControlID:          normalizeTimeControlID(timeControlID),
		TimeControlLabel:       timeControlLabel,
		TimeControlBaseMs:      timeControlBaseMs,
		TimeControlIncrementMs: timeControlIncrementMs,
		Player1RemainingMs:     player1RemainingMs,
		Player2RemainingMs:     player2RemainingMs,
		CurrentTurnStartedAt:   formatTimeControlTimestamp(currentTurnStartedAt),
	}
}

func buildTimeControlPayloadFromPreset(timeControlID string) timeControlPayload {
	timeControl, ok := resolveTimeControl(timeControlID)
	if !ok {
		timeControl = TimeControl{ID: TimeControlUnlimited}
	}

	baseMs := durationToMilliseconds(timeControl.Base)
	incrementMs := durationToMilliseconds(timeControl.Increment)

	return buildTimeControlPayload(
		timeControl.ID,
		timeControl.Label,
		baseMs,
		incrementMs,
		baseMs,
		baseMs,
		time.Time{},
	)
}

func formatTimeControlTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.UTC().Format(time.RFC3339Nano)
}

func (s State) MarshalJSON() ([]byte, error) {
	type stateAlias State

	return json.Marshal(struct {
		stateAlias
		TimeControlID          string `json:"time_control_id"`
		TimeControlLabel       string `json:"time_control_label"`
		TimeControlBaseMs      int64  `json:"time_control_base_ms"`
		TimeControlIncrementMs int64  `json:"time_control_increment_ms"`
		Player1RemainingMs     int64  `json:"player1_remaining_ms"`
		Player2RemainingMs     int64  `json:"player2_remaining_ms"`
		CurrentTurnStartedAt   string `json:"current_turn_started_at"`
	}{
		stateAlias:             stateAlias(s),
		TimeControlID:          normalizeTimeControlID(s.TimeControlID),
		TimeControlLabel:       s.TimeControlLabel,
		TimeControlBaseMs:      s.TimeControlBaseMs,
		TimeControlIncrementMs: s.TimeControlIncrementMs,
		Player1RemainingMs:     s.Player1RemainingMs,
		Player2RemainingMs:     s.Player2RemainingMs,
		CurrentTurnStartedAt:   formatTimeControlTimestamp(s.CurrentTurnStartedAt),
	})
}
