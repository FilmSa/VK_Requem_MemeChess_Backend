package game

import (
	"strings"
	"time"
)

const TimeControlUnlimited = "unlimited"

type TimeControl struct {
	ID        string
	Label     string
	Base      time.Duration
	Increment time.Duration
}

var timeControlsByID = map[string]TimeControl{
	"classic": {
		ID:        "classic",
		Label:     "30+9",
		Base:      30 * time.Minute,
		Increment: 9 * time.Second,
	},
	"rapid": {
		ID:        "rapid",
		Label:     "15+9",
		Base:      15 * time.Minute,
		Increment: 9 * time.Second,
	},
	"blitz": {
		ID:        "blitz",
		Label:     "3+2",
		Base:      3 * time.Minute,
		Increment: 2 * time.Second,
	},
	"bullet": {
		ID:        "bullet",
		Label:     "1+5",
		Base:      1 * time.Minute,
		Increment: 5 * time.Second,
	},
}

func resolveTimeControl(timeControlID string) (TimeControl, bool) {
	switch normalizeTimeControlID(timeControlID) {
	case "", TimeControlUnlimited:
		return TimeControl{ID: TimeControlUnlimited, Label: "", Base: 0, Increment: 0}, true
	case "classic", "rapid", "blitz", "bullet":
		timeControl := timeControlsByID[normalizeTimeControlID(timeControlID)]
		return timeControl, true
	default:
		return TimeControl{}, false
	}
}

func normalizeTimeControlID(timeControlID string) string {
	switch trimLower(timeControlID) {
	case "", TimeControlUnlimited:
		return TimeControlUnlimited
	case "classic", "rapid", "blitz", "bullet":
		return trimLower(timeControlID)
	default:
		return ""
	}
}

func durationToMilliseconds(value time.Duration) int64 {
	if value <= 0 {
		return 0
	}

	return value.Milliseconds()
}

func trimLower(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
