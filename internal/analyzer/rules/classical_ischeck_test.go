package rules

import (
	"testing"

	"meme_chess/internal/analyzer/position"
)

func TestClassicalRuleSetDetectsFoolsMateCheck(t *testing.T) {
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

	if !NewClassicalRuleSet().IsCheck(gs, position.White) {
		t.Fatal("expected white king to be in check after Qh4")
	}
	if NewClassicalRuleSet().IsCheck(gs, position.Black) {
		t.Fatal("did not expect black king to be in check after Qh4")
	}
}
