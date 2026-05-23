package search

import (
	"testing"

	"meme_chess/internal/analyzer/movegen"
	"meme_chess/internal/analyzer/position"
	"meme_chess/internal/analyzer/rules"
)

func TestAnalyzePositionPreservesRootStateAndAllRootMoves(t *testing.T) {
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
	rs := rules.NewClassicalRuleSet()
	expected := movegen.NewGenerator(rs).GenerateLegalMoves(gs)
	result := NewEngine(rs).AnalyzePosition(gs, 4)

	if after := gs.FEN(); after != before {
		t.Fatalf("expected search to preserve root state, before=%s after=%s", before, after)
	}
	if len(result.RootMoves) != len(expected) {
		t.Fatalf("expected %d root moves, got %d", len(expected), len(result.RootMoves))
	}
}

func TestNegamaxPreservesChildState(t *testing.T) {
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

	rootMove, err := position.ParseUCIMove(gs, "d8h4")
	if err != nil {
		t.Fatalf("parse root move: %v", err)
	}
	if err := gs.ApplyMove(rootMove); err != nil {
		t.Fatalf("apply root move: %v", err)
	}

	childBefore := gs.FEN()
	engine := NewEngine(rules.NewClassicalRuleSet())
	nodes := 0
	engine.negamax(gs, NewTranspositionTable(), 4, 1, negInf, posInf, &nodes)
	if childAfter := gs.FEN(); childAfter != childBefore {
		t.Fatalf("expected negamax to preserve child state, before=%s after=%s", childBefore, childAfter)
	}
}

func TestNegamaxPreservesChildStateOnDevelopingMove(t *testing.T) {
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

	rootMove, err := position.ParseUCIMove(gs, "b8c6")
	if err != nil {
		t.Fatalf("parse root move: %v", err)
	}
	if err := gs.ApplyMove(rootMove); err != nil {
		t.Fatalf("apply root move: %v", err)
	}

	childBefore := gs.FEN()
	engine := NewEngine(rules.NewClassicalRuleSet())
	nodes := 0
	engine.negamax(gs, NewTranspositionTable(), 4, 1, negInf, posInf, &nodes)
	if childAfter := gs.FEN(); childAfter != childBefore {
		t.Fatalf("expected negamax to preserve child state, before=%s after=%s", childBefore, childAfter)
	}
}

func TestQuiescencePreservesStateOnDevelopingMove(t *testing.T) {
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
	engine := NewEngine(rules.NewClassicalRuleSet())
	nodes := 0
	_ = engine.quiescence(gs, 0, negInf, posInf, &nodes)
	if after := gs.FEN(); after != before {
		t.Fatalf("expected quiescence to preserve state, before=%s after=%s", before, after)
	}
}
