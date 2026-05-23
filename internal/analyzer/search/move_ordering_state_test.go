package search

import (
	"testing"

	"meme_chess/internal/analyzer/movegen"
	"meme_chess/internal/analyzer/position"
	"meme_chess/internal/analyzer/rules"
)

func TestMoveOrderingPreservesStateAndKeepsMateMove(t *testing.T) {
	gs := position.NewInitial()
	for _, raw := range []string{"f2f3", "e7e5", "g2g4"} {
		mv, err := position.ParseUCIMove(gs, raw)
		if err != nil {
			t.Fatalf("parse move %s: %v", raw, err)
		}
		if err := gs.ApplyMove(mv); err != nil {
			t.Fatalf("apply move %s: %v", raw, err)
		}
	}

	before := gs.FEN()
	gen := movegen.NewGenerator(rules.NewClassicalRuleSet())
	moves := gen.GenerateLegalMoves(gs)
	ordered := NewMoveOrdering(rules.NewClassicalRuleSet()).Order(gs, moves, position.NullMove())

	if after := gs.FEN(); after != before {
		t.Fatalf("expected ordering to preserve position, before=%s after=%s", before, after)
	}

	for _, mv := range ordered {
		if mv.From == position.MustSquare(3, 7) && mv.To == position.MustSquare(7, 3) {
			return
		}
	}

	t.Fatal("expected ordered root moves to include d8h4")
}
