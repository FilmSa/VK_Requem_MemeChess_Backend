package position

import "testing"

func TestNewInitialFEN(t *testing.T) {
	gs := NewInitial()
	got := gs.FEN()
	want := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	if got != want {
		t.Fatalf("unexpected FEN\nwant: %s\ngot:  %s", want, got)
	}
}

func TestApplyMove(t *testing.T) {
	gs := NewInitial()
	mv, err := NewMove("e2", "e4", NoPieceType)
	if err != nil {
		t.Fatalf("NewMove error: %v", err)
	}

	if err := gs.ApplyMove(mv); err != nil {
		t.Fatalf("ApplyMove error: %v", err)
	}

	if piece := gs.PieceAt(MustSquare(4, 3)); piece.Type != Pawn || piece.Color != White {
		t.Fatalf("expected white pawn on e4, got %+v", piece)
	}
	if piece := gs.PieceAt(MustSquare(4, 1)); !piece.IsZero() {
		t.Fatalf("expected e2 to be empty, got %+v", piece)
	}
	if gs.SideToMove != Black {
		t.Fatalf("expected black to move, got %v", gs.SideToMove)
	}
}

func TestUndoMove(t *testing.T) {
	gs := NewInitial()
	before := gs.FEN()

	mv, _ := NewMove("e2", "e4", NoPieceType)
	if err := gs.ApplyMove(mv); err != nil {
		t.Fatalf("ApplyMove error: %v", err)
	}
	if err := gs.UndoMove(); err != nil {
		t.Fatalf("UndoMove error: %v", err)
	}

	after := gs.FEN()
	if before != after {
		t.Fatalf("state mismatch after undo\nwant: %s\ngot:  %s", before, after)
	}
}

func TestUndoMoveAfterCastling(t *testing.T) {
	gs := NewInitial()
	for _, raw := range []string{"g1f3", "g8f6", "f1c4", "b8c6"} {
		mv, err := ParseUCIMove(gs, raw)
		if err != nil {
			t.Fatalf("parse move %s: %v", raw, err)
		}
		if err := gs.ApplyMove(mv); err != nil {
			t.Fatalf("apply move %s: %v", raw, err)
		}
	}

	before := gs.FEN()
	castle, err := ParseUCIMove(gs, "e1g1")
	if err != nil {
		t.Fatalf("parse castle: %v", err)
	}
	if err := gs.ApplyMove(castle); err != nil {
		t.Fatalf("apply castle: %v", err)
	}
	if err := gs.UndoMove(); err != nil {
		t.Fatalf("undo castle: %v", err)
	}

	after := gs.FEN()
	if before != after {
		t.Fatalf("state mismatch after castling undo\nwant: %s\ngot:  %s", before, after)
	}
}

func TestUndoMoveAfterCastlingSequence(t *testing.T) {
	gs := NewInitial()
	for _, raw := range []string{
		"f2f3", "e7e5", "g2g4", "b8c6",
		"g1h3", "g8f6", "f1g2", "f8c5",
		"e1g1", "e8g8",
	} {
		mv, err := ParseUCIMove(gs, raw)
		if err != nil {
			t.Fatalf("parse move %s: %v", raw, err)
		}
		if err := gs.ApplyMove(mv); err != nil {
			t.Fatalf("apply move %s: %v", raw, err)
		}
	}

	for i := 0; i < 10; i++ {
		if err := gs.UndoMove(); err != nil {
			t.Fatalf("undo step %d: %v", i, err)
		}
	}

	want := NewInitial().FEN()
	if got := gs.FEN(); got != want {
		t.Fatalf("state mismatch after castling sequence undo\nwant: %s\ngot:  %s", want, got)
	}
}

func TestHashStableForEqualPositions(t *testing.T) {
	gs1 := NewInitial()
	gs2 := NewInitial()

	if gs1.Hash() != gs2.Hash() {
		t.Fatalf("expected equal hashes for equal positions")
	}
}

func TestMoveFields(t *testing.T) {
	mv, err := NewMove("a7", "a8", Queen)
	if err != nil {
		t.Fatalf("NewMove error: %v", err)
	}

	if mv.From.String() != "a7" {
		t.Fatalf("unexpected from: %s", mv.From)
	}
	if mv.To.String() != "a8" {
		t.Fatalf("unexpected to: %s", mv.To)
	}
	if mv.Promotion != Queen {
		t.Fatalf("unexpected promotion: %d", mv.Promotion)
	}
	if mv.Kind != MovePromotion {
		t.Fatalf("expected promotion kind, got %d", mv.Kind)
	}
}
