package movegen

import (
	"testing"

	"meme_chess/internal/analyzer/position"
	"meme_chess/internal/analyzer/rules"
)

func TestGenerateLegalMovesIncludesMateInOneQh4(t *testing.T) {
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
	moves := NewGenerator(rules.NewClassicalRuleSet()).GenerateLegalMoves(gs)
	if after := gs.FEN(); after != before {
		t.Fatalf("expected GenerateLegalMoves to preserve state, before=%s after=%s", before, after)
	}
	for _, mv := range moves {
		if mv.From == position.MustSquare(3, 7) && mv.To == position.MustSquare(7, 3) {
			return
		}
	}

	t.Fatal("expected d8h4 to be generated as a legal move")
}

func TestGenerateLegalMovesPreservesDevelopingPositionState(t *testing.T) {
	gs := position.NewInitial()
	for _, raw := range []string{"f2f3", "e7e5", "g2g4", "b8c6"} {
		mv, err := position.ParseUCIMove(gs, raw)
		if err != nil {
			t.Fatalf("parse move %s: %v", raw, err)
		}
		if err := gs.ApplyMove(mv); err != nil {
			t.Fatalf("apply move %s: %v", raw, err)
		}
	}

	before := gs.FEN()
	_ = NewGenerator(rules.NewClassicalRuleSet()).GenerateLegalMoves(gs)
	if after := gs.FEN(); after != before {
		t.Fatalf("expected GenerateLegalMoves to preserve state, before=%s after=%s", before, after)
	}
}
