package game

import (
	"context"
	"testing"

	"meme_chess/internal/analyzer/position"
	"meme_chess/internal/analyzer/rules"
)

type stubRandomizer struct {
	values []int
	index  int
}

func (s *stubRandomizer) Intn(n int) (int, error) {
	if len(s.values) == 0 {
		return 0, nil
	}
	value := s.values[s.index%len(s.values)]
	s.index++
	if n == 0 {
		return 0, nil
	}
	return value % n, nil
}

func TestCreateGameWithModeStoresGameMode(t *testing.T) {
	svc := NewService(nil)
	engine, err := NewChessEngineForMode(GameModeFischer)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	session, err := svc.CreateGameWithMode(context.Background(), "game-fischer", GameModeFischer, "u1", "u2", engine)
	if err != nil {
		t.Fatalf("create game: %v", err)
	}

	if got := session.Snapshot().GameMode; got != GameModeFischer {
		t.Fatalf("expected game mode %q, got %q", GameModeFischer, got)
	}
}

func TestFischerEngineBuildsValidBackRank(t *testing.T) {
	engine, err := newChessEngineForMode(GameModeFischer, &stubRandomizer{values: []int{0, 0, 0, 0, 0}})
	if err != nil {
		t.Fatalf("new fischer engine: %v", err)
	}

	runtime := engine.runtime.(*analyzerRuntime)
	layout := runtime.state.CastlingLayoutValue()

	bishops := make([]position.Square, 0, 2)
	rooks := make([]position.Square, 0, 2)
	var king position.Square

	for file := 0; file < 8; file++ {
		sq := position.MustSquare(file, 0)
		piece := runtime.state.PieceAt(sq)
		if piece.Color != position.White {
			t.Fatalf("expected white piece at %s, got %+v", sq, piece)
		}
		switch piece.Type {
		case position.Bishop:
			bishops = append(bishops, sq)
		case position.Rook:
			rooks = append(rooks, sq)
		case position.King:
			king = sq
		}

		black := runtime.state.PieceAt(position.MustSquare(file, 7))
		if black.Type != piece.Type || black.Color != position.Black {
			t.Fatalf("expected mirrored black piece at file %d, got white=%+v black=%+v", file, piece, black)
		}
	}

	if len(bishops) != 2 || len(rooks) != 2 {
		t.Fatalf("expected 2 bishops and 2 rooks, got bishops=%d rooks=%d", len(bishops), len(rooks))
	}
	if bishops[0].File()%2 == bishops[1].File()%2 {
		t.Fatalf("expected bishops on opposite colors, got %s and %s", bishops[0], bishops[1])
	}
	if !(rooks[0].File() < king.File() && king.File() < rooks[1].File()) {
		t.Fatalf("expected king between rooks, got rooks=%v king=%s", rooks, king)
	}
	if layout.KingStart(position.White) != king {
		t.Fatalf("expected layout king start %s, got %s", king, layout.KingStart(position.White))
	}
}

func TestFischerCastlingMovesKingAndRookToStandardSquares(t *testing.T) {
	state := emptyState(position.White)
	state.SetPiece(position.MustSquare(2, 0), position.Piece{Type: position.King, Color: position.White})
	state.SetPiece(position.MustSquare(7, 0), position.Piece{Type: position.Rook, Color: position.White})
	state.SetPiece(position.MustSquare(4, 7), position.Piece{Type: position.King, Color: position.Black})
	state.CastlingRights.WhiteKingSide = true
	state.CastlingRights.WhiteQueenSide = false
	state.CastlingRights.BlackKingSide = false
	state.CastlingRights.BlackQueenSide = false
	state.CastlingLayout = &position.CastlingLayout{
		White: position.CastlingSideLayout{
			KingStart:          position.MustSquare(2, 0),
			KingSideRookStart:  position.MustSquare(7, 0),
			QueenSideRookStart: position.MustSquare(0, 0),
		},
		Black: position.CastlingSideLayout{
			KingStart:          position.MustSquare(4, 7),
			KingSideRookStart:  position.MustSquare(7, 7),
			QueenSideRookStart: position.MustSquare(0, 7),
		},
	}

	result, err := applySingleMove(state, rules.NewClassicalRuleSet(), "c1g1")
	if err != nil {
		t.Fatalf("castle failed: %v", err)
	}
	if got := state.PieceAt(position.MustSquare(6, 0)); got.Type != position.King || got.Color != position.White {
		t.Fatalf("expected white king on g1, got %+v", got)
	}
	if got := state.PieceAt(position.MustSquare(5, 0)); got.Type != position.Rook || got.Color != position.White {
		t.Fatalf("expected white rook on f1, got %+v", got)
	}
	if result.Move != "c1g1" {
		t.Fatalf("expected normalized move c1g1, got %q", result.Move)
	}
}

func TestFischerCastleAllowsStationaryKingNotation(t *testing.T) {
	state := emptyState(position.White)
	state.SetPiece(position.MustSquare(6, 0), position.Piece{Type: position.King, Color: position.White})
	state.SetPiece(position.MustSquare(7, 0), position.Piece{Type: position.Rook, Color: position.White})
	state.SetPiece(position.MustSquare(4, 7), position.Piece{Type: position.King, Color: position.Black})
	state.CastlingRights.WhiteKingSide = true
	state.CastlingRights.WhiteQueenSide = false
	state.CastlingRights.BlackKingSide = false
	state.CastlingRights.BlackQueenSide = false
	state.CastlingLayout = &position.CastlingLayout{
		White: position.CastlingSideLayout{
			KingStart:          position.MustSquare(6, 0),
			KingSideRookStart:  position.MustSquare(7, 0),
			QueenSideRookStart: position.MustSquare(0, 0),
		},
		Black: position.CastlingSideLayout{
			KingStart:          position.MustSquare(4, 7),
			KingSideRookStart:  position.MustSquare(7, 7),
			QueenSideRookStart: position.MustSquare(0, 7),
		},
	}

	result, err := applySingleMove(state, rules.NewClassicalRuleSet(), "g1g1")
	if err != nil {
		t.Fatalf("castle failed: %v", err)
	}
	if got := state.PieceAt(position.MustSquare(6, 0)); got.Type != position.King || got.Color != position.White {
		t.Fatalf("expected white king to remain on g1, got %+v", got)
	}
	if got := state.PieceAt(position.MustSquare(5, 0)); got.Type != position.Rook || got.Color != position.White {
		t.Fatalf("expected white rook on f1, got %+v", got)
	}
	if result.Move != "g1g1" {
		t.Fatalf("expected normalized move g1g1, got %q", result.Move)
	}
}

func TestEvolutionPawnCounterRemovesAttackingPawn(t *testing.T) {
	runtime := &evolutionRuntime{
		state: emptyState(position.White),
		turns: 5,
		rng:   &stubRandomizer{values: []int{0}},
	}
	runtime.state.SetPiece(position.MustSquare(4, 0), position.Piece{Type: position.King, Color: position.White})
	runtime.state.SetPiece(position.MustSquare(4, 7), position.Piece{Type: position.King, Color: position.Black})
	runtime.state.SetPiece(position.MustSquare(4, 3), position.Piece{Type: position.Pawn, Color: position.White})
	runtime.state.SetPiece(position.MustSquare(3, 4), position.Piece{Type: position.Pawn, Color: position.Black})

	result, err := runtime.ApplyMove("e4d5")
	if err != nil {
		t.Fatalf("apply move: %v", err)
	}
	if !result.IsCapture {
		t.Fatal("expected capture to be recorded")
	}
	if piece := runtime.state.PieceAt(position.MustSquare(3, 4)); !piece.IsZero() {
		t.Fatalf("expected attacker square to be empty after counter, got %+v", piece)
	}
}

func TestEvolutionKingRevengeCancelsSinglePieceMate(t *testing.T) {
	state := position.NewInitial()
	for _, move := range []string{"f2f3", "e7e5", "g2g4"} {
		mv, err := position.ParseUCIMove(state, move)
		if err != nil {
			t.Fatalf("parse %s: %v", move, err)
		}
		if err := state.ApplyMove(mv); err != nil {
			t.Fatalf("apply %s: %v", move, err)
		}
	}

	runtime := &evolutionRuntime{
		state: state,
		turns: 7,
		rng:   &stubRandomizer{},
	}

	result, err := runtime.ApplyMove("d8h4")
	if err != nil {
		t.Fatalf("apply move: %v", err)
	}
	if result.IsCheckmate {
		t.Fatal("expected king revenge to cancel checkmate")
	}
	if piece := runtime.state.PieceAt(position.MustSquare(7, 3)); !piece.IsZero() {
		t.Fatalf("expected mating queen to be removed from h4, got %+v", piece)
	}
}

func TestEvolutionKingRevengeOnlyWorksOncePerColor(t *testing.T) {
	state := position.NewInitial()
	for _, move := range []string{"f2f3", "e7e5", "g2g4"} {
		mv, err := position.ParseUCIMove(state, move)
		if err != nil {
			t.Fatalf("parse %s: %v", move, err)
		}
		if err := state.ApplyMove(mv); err != nil {
			t.Fatalf("apply %s: %v", move, err)
		}
	}

	runtime := &evolutionRuntime{
		state: state,
		turns: 7,
		kingRevengeUsed: map[position.Color]bool{
			position.White: true,
		},
		rng: &stubRandomizer{},
	}

	result, err := runtime.ApplyMove("d8h4")
	if err != nil {
		t.Fatalf("apply move: %v", err)
	}
	if !result.IsCheckmate {
		t.Fatal("expected checkmate once king revenge was already spent")
	}
	if piece := runtime.state.PieceAt(position.MustSquare(7, 3)); piece.Type != position.Queen || piece.Color != position.Black {
		t.Fatalf("expected mating queen to remain on h4, got %+v", piece)
	}
}

func TestEvolutionKnightCanMoveTwiceInOneTurn(t *testing.T) {
	runtime := &evolutionRuntime{
		state: emptyState(position.White),
		turns: 10,
		rng:   &stubRandomizer{},
	}
	runtime.state.SetPiece(position.MustSquare(4, 0), position.Piece{Type: position.King, Color: position.White})
	runtime.state.SetPiece(position.MustSquare(4, 7), position.Piece{Type: position.King, Color: position.Black})
	runtime.state.SetPiece(position.MustSquare(6, 0), position.Piece{Type: position.Knight, Color: position.White})

	result, err := runtime.ApplyMove("g1f3,f3e5")
	if err != nil {
		t.Fatalf("apply move: %v", err)
	}
	if result.Move != "g1f3,f3e5" {
		t.Fatalf("expected combined move, got %q", result.Move)
	}
	if piece := runtime.state.PieceAt(position.MustSquare(4, 4)); piece.Type != position.Knight || piece.Color != position.White {
		t.Fatalf("expected knight on e5, got %+v", piece)
	}
}

func TestEvolutionBishopPiercesThroughPawn(t *testing.T) {
	runtime := &evolutionRuntime{
		state: emptyState(position.White),
		turns: 15,
		rng:   &stubRandomizer{},
	}
	runtime.state.SetPiece(position.MustSquare(7, 0), position.Piece{Type: position.King, Color: position.White})
	runtime.state.SetPiece(position.MustSquare(0, 7), position.Piece{Type: position.King, Color: position.Black})
	runtime.state.SetPiece(position.MustSquare(2, 0), position.Piece{Type: position.Bishop, Color: position.White})
	runtime.state.SetPiece(position.MustSquare(3, 1), position.Piece{Type: position.Pawn, Color: position.Black})
	runtime.state.SetPiece(position.MustSquare(4, 2), position.Piece{Type: position.Knight, Color: position.Black})

	result, err := runtime.ApplyMove("c1e3")
	if err != nil {
		t.Fatalf("apply move: %v", err)
	}
	if !result.IsCapture {
		t.Fatal("expected capture to be recorded")
	}
	if piece := runtime.state.PieceAt(position.MustSquare(4, 2)); piece.Type != position.Bishop || piece.Color != position.White {
		t.Fatalf("expected bishop on e3, got %+v", piece)
	}
	if piece := runtime.state.PieceAt(position.MustSquare(3, 1)); piece.Type != position.Pawn || piece.Color != position.Black {
		t.Fatalf("expected pawn on d2 to remain, got %+v", piece)
	}
}

func TestEvolutionRookRampageClearsPiecesOnPath(t *testing.T) {
	runtime := &evolutionRuntime{
		state: emptyState(position.White),
		turns: 20,
		rng:   &stubRandomizer{},
	}
	runtime.state.SetPiece(position.MustSquare(4, 0), position.Piece{Type: position.King, Color: position.White})
	runtime.state.SetPiece(position.MustSquare(4, 7), position.Piece{Type: position.King, Color: position.Black})
	runtime.state.SetPiece(position.MustSquare(0, 0), position.Piece{Type: position.Rook, Color: position.White})
	runtime.state.SetPiece(position.MustSquare(0, 1), position.Piece{Type: position.Pawn, Color: position.White})
	runtime.state.SetPiece(position.MustSquare(0, 2), position.Piece{Type: position.Pawn, Color: position.Black})

	result, err := runtime.ApplyMove("a1a4")
	if err != nil {
		t.Fatalf("apply move: %v", err)
	}
	if !result.IsCapture {
		t.Fatal("expected rampage move to count as capture")
	}
	if piece := runtime.state.PieceAt(position.MustSquare(0, 3)); piece.Type != position.Rook || piece.Color != position.White {
		t.Fatalf("expected rook on a4, got %+v", piece)
	}
	if piece := runtime.state.PieceAt(position.MustSquare(0, 1)); !piece.IsZero() {
		t.Fatalf("expected friendly pawn on a2 to be removed, got %+v", piece)
	}
	if piece := runtime.state.PieceAt(position.MustSquare(0, 2)); !piece.IsZero() {
		t.Fatalf("expected enemy pawn on a3 to be removed, got %+v", piece)
	}
}

func emptyState(side position.Color) *position.GameState {
	return &position.GameState{
		SideToMove: side,
		CastlingRights: position.CastlingRights{
			WhiteKingSide:  false,
			WhiteQueenSide: false,
			BlackKingSide:  false,
			BlackQueenSide: false,
		},
		CastlingLayout: position.StandardCastlingLayout(),
		EnPassant:      position.NoSquare,
		HalfmoveClock:  0,
		FullmoveNumber: 1,
	}
}
