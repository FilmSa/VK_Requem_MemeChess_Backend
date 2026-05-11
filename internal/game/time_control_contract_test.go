package game

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStateMarshalJSON_IncludesCanonicalTimeControlFields(t *testing.T) {
	startedAt := time.Date(2026, time.May, 11, 12, 34, 56, 789000000, time.UTC)
	state := State{
		GameID:                 "game-1",
		Status:                 string(StatusActive),
		TimeControlID:          "rapid",
		TimeControlLabel:       "15+9",
		TimeControlBaseMs:      15 * 60 * 1000,
		TimeControlIncrementMs: 9 * 1000,
		Player1RemainingMs:     15 * 60 * 1000,
		Player2RemainingMs:     15 * 60 * 1000,
		CurrentTurnStartedAt:   startedAt,
	}

	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal state payload: %v", err)
	}

	if payload["time_control_id"] != "rapid" {
		t.Fatalf("expected canonical time_control_id, got %#v", payload["time_control_id"])
	}
	if payload["time_control_label"] != "15+9" {
		t.Fatalf("expected time_control_label, got %#v", payload["time_control_label"])
	}
	if payload["current_turn_started_at"] != startedAt.Format(time.RFC3339Nano) {
		t.Fatalf(
			"expected current_turn_started_at %q, got %#v",
			startedAt.Format(time.RFC3339Nano),
			payload["current_turn_started_at"],
		)
	}
}

func TestStateMarshalJSON_UsesEmptyTimestampForPausedClock(t *testing.T) {
	state := State{
		GameID:                 "game-2",
		Status:                 string(StatusActive),
		TimeControlID:          "blitz",
		TimeControlLabel:       "3+2",
		TimeControlBaseMs:      3 * 60 * 1000,
		TimeControlIncrementMs: 2 * 1000,
		Player1RemainingMs:     3 * 60 * 1000,
		Player2RemainingMs:     3 * 60 * 1000,
	}

	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal state payload: %v", err)
	}

	if _, ok := payload["time_control_base_ms"]; !ok {
		t.Fatal("expected time_control_base_ms to be present")
	}
	if payload["current_turn_started_at"] != "" {
		t.Fatalf(
			"expected empty current_turn_started_at for paused clock, got %#v",
			payload["current_turn_started_at"],
		)
	}
}
