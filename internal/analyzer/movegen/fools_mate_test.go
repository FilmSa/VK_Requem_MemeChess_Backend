package movegen

import (
	"testing"

	"meme_chess/internal/analyzer/position"
	"meme_chess/internal/analyzer/rules"
)

func TestGenerateLegalMovesInFoolsMatePosition(t *testing.T) {
	gs := position.NewInitial()
	for _, raw := range []string{"f2f3", "e7e5", "g2g4", "d8h4"} {
		mv, err := position.ParseUCIMove(gs, raw)
		if err != nil {
			t.Fatalf("parse move %s: %v", raw, err)
		}
		if err := gs.ApplyMove(mv); err != nil {
			t.Fatalf("apply move %s: %v", raw, err)
		}
	}

	moves := NewGenerator(rules.NewClassicalRuleSet()).GenerateLegalMoves(gs)
	if len(moves) != 0 {
		t.Fatalf("expected no legal replies in fool's mate, got %d", len(moves))
	}
}
