package position

import "fmt"

type CastlingRights struct {
	WhiteKingSide  bool
	WhiteQueenSide bool
	BlackKingSide  bool
	BlackQueenSide bool
}

func (c CastlingRights) String() string {
	s := ""
	if c.WhiteKingSide {
		s += "K"
	}
	if c.WhiteQueenSide {
		s += "Q"
	}
	if c.BlackKingSide {
		s += "k"
	}
	if c.BlackQueenSide {
		s += "q"
	}
	if s == "" {
		return "-"
	}
	return s
}

type Undo struct {
	IsNull         bool
	Move           Move
	MovedPiece     Piece
	CapturedPiece  Piece
	CapturedSquare Square
	RookFrom       Square
	RookTo         Square
	PrevEnPassant  Square
	PrevCastling   CastlingRights
	PrevHalfmove   int
	PrevFullmove   int
	PrevSideToMove Color
}

type GameState struct {
	Board           [64]Piece
	SideToMove      Color
	CastlingRights  CastlingRights
	CastlingLayout  *CastlingLayout
	EnPassant       Square
	HalfmoveClock   int
	FullmoveNumber  int
	WhiteKingSquare Square
	BlackKingSquare Square
	History         []Undo
}

func NewInitial() *GameState {
	gs := &GameState{
		SideToMove: White,
		CastlingRights: CastlingRights{
			WhiteKingSide:  true,
			WhiteQueenSide: true,
			BlackKingSide:  true,
			BlackQueenSide: true,
		},
		CastlingLayout:  StandardCastlingLayout(),
		EnPassant:       NoSquare,
		HalfmoveClock:   0,
		FullmoveNumber:  1,
		WhiteKingSquare: NoSquare,
		BlackKingSquare: NoSquare,
	}

	backRank := [8]PieceType{Rook, Knight, Bishop, Queen, King, Bishop, Knight, Rook}
	for file := 0; file < 8; file++ {
		gs.SetPiece(Square(file), Piece{Type: backRank[file], Color: White})
		gs.SetPiece(Square(8+file), Piece{Type: Pawn, Color: White})
		gs.SetPiece(Square(6*8+file), Piece{Type: Pawn, Color: Black})
		gs.SetPiece(Square(7*8+file), Piece{Type: backRank[file], Color: Black})
	}

	return gs
}

func (g *GameState) Clone() *GameState {
	cp := *g
	if g.CastlingLayout != nil {
		layout := *g.CastlingLayout
		cp.CastlingLayout = &layout
	}
	cp.History = append([]Undo(nil), g.History...)
	return &cp
}

func (g *GameState) PieceAt(sq Square) Piece {
	if sq == NoSquare {
		return Piece{}
	}
	return g.Board[sq]
}

func (g *GameState) SetPiece(sq Square, p Piece) {
	if sq == NoSquare {
		return
	}

	current := g.Board[sq]
	if current.Type == King {
		if current.Color == White && g.WhiteKingSquare == sq {
			g.WhiteKingSquare = NoSquare
		}
		if current.Color == Black && g.BlackKingSquare == sq {
			g.BlackKingSquare = NoSquare
		}
	}

	g.Board[sq] = p
	if p.Type == King {
		if p.Color == White {
			g.WhiteKingSquare = sq
		} else {
			g.BlackKingSquare = sq
		}
	}
}

func (g *GameState) KingSquare(color Color) (Square, bool) {
	switch color {
	case White:
		return g.WhiteKingSquare, g.WhiteKingSquare != NoSquare
	case Black:
		return g.BlackKingSquare, g.BlackKingSquare != NoSquare
	default:
		return NoSquare, false
	}
}

func (g *GameState) ApplyMove(m Move) error {
	moved := g.PieceAt(m.From)
	if moved.IsZero() {
		return fmt.Errorf("no piece at source square %s", m.From)
	}
	if moved.Color != g.SideToMove {
		return fmt.Errorf("piece at %s does not belong to side to move", m.From)
	}

	undo := Undo{
		Move:           m,
		MovedPiece:     moved,
		PrevEnPassant:  g.EnPassant,
		PrevCastling:   g.CastlingRights,
		PrevHalfmove:   g.HalfmoveClock,
		PrevFullmove:   g.FullmoveNumber,
		PrevSideToMove: g.SideToMove,
		CapturedSquare: m.To,
		RookFrom:       NoSquare,
		RookTo:         NoSquare,
	}

	g.EnPassant = NoSquare

	switch m.Kind {
	case MoveEnPassant:
		undo.CapturedSquare = enPassantCapturedSquare(m.To, moved.Color)
		undo.CapturedPiece = g.PieceAt(undo.CapturedSquare)

		g.SetPiece(m.From, Piece{})
		g.SetPiece(undo.CapturedSquare, Piece{})
		g.SetPiece(m.To, moved)

	case MoveCastleKingSide:
		layout := g.CastlingLayoutValue()
		undo.RookFrom = layout.RookStart(moved.Color, m.Kind)
		undo.RookTo = layout.RookEnd(moved.Color, m.Kind)
		rook := g.PieceAt(undo.RookFrom)
		g.SetPiece(m.From, Piece{})
		g.SetPiece(undo.RookFrom, Piece{})
		g.SetPiece(m.To, moved)
		g.SetPiece(undo.RookTo, rook)

	case MoveCastleQueenSide:
		layout := g.CastlingLayoutValue()
		undo.RookFrom = layout.RookStart(moved.Color, m.Kind)
		undo.RookTo = layout.RookEnd(moved.Color, m.Kind)
		rook := g.PieceAt(undo.RookFrom)
		g.SetPiece(m.From, Piece{})
		g.SetPiece(undo.RookFrom, Piece{})
		g.SetPiece(m.To, moved)
		g.SetPiece(undo.RookTo, rook)

	default:
		undo.CapturedPiece = g.PieceAt(m.To)
		g.SetPiece(m.From, Piece{})

		if m.Promotion != NoPieceType {
			g.SetPiece(m.To, Piece{Type: m.Promotion, Color: moved.Color})
		} else {
			g.SetPiece(m.To, moved)
		}

		if moved.Type == Pawn && abs(m.To.Rank()-m.From.Rank()) == 2 {
			midRank := (m.To.Rank() + m.From.Rank()) / 2
			g.EnPassant = MustSquare(m.From.File(), midRank)
		}
	}

	g.History = append(g.History, undo)
	g.updateCastlingRights(m, moved, undo.CapturedPiece, undo.CapturedSquare)

	if moved.Type == Pawn || !undo.CapturedPiece.IsZero() {
		g.HalfmoveClock = 0
	} else {
		g.HalfmoveClock++
	}

	if moved.Color == Black {
		g.FullmoveNumber++
	}
	g.SideToMove = g.SideToMove.Opponent()

	return nil
}

func (g *GameState) ApplyNullMove() {
	undo := Undo{
		IsNull:         true,
		Move:           NullMove(),
		PrevEnPassant:  g.EnPassant,
		PrevCastling:   g.CastlingRights,
		PrevHalfmove:   g.HalfmoveClock,
		PrevFullmove:   g.FullmoveNumber,
		PrevSideToMove: g.SideToMove,
		CapturedSquare: NoSquare,
		RookFrom:       NoSquare,
		RookTo:         NoSquare,
	}

	g.EnPassant = NoSquare
	if g.SideToMove == Black {
		g.FullmoveNumber++
	}
	g.SideToMove = g.SideToMove.Opponent()
	g.History = append(g.History, undo)
}

func (g *GameState) UndoMove() error {
	if len(g.History) == 0 {
		return fmt.Errorf("no moves to undo")
	}

	last := g.History[len(g.History)-1]
	g.History = g.History[:len(g.History)-1]

	g.SideToMove = last.PrevSideToMove
	g.CastlingRights = last.PrevCastling
	g.EnPassant = last.PrevEnPassant
	g.HalfmoveClock = last.PrevHalfmove
	g.FullmoveNumber = last.PrevFullmove

	if last.IsNull {
		return nil
	}

	if last.Move.Kind == MoveCastleKingSide || last.Move.Kind == MoveCastleQueenSide {
		rook := g.PieceAt(last.RookTo)
		g.SetPiece(last.Move.To, Piece{})
		g.SetPiece(last.RookTo, Piece{})
		g.SetPiece(last.Move.From, last.MovedPiece)
		g.SetPiece(last.RookFrom, rook)
		return nil
	}

	g.SetPiece(last.Move.From, last.MovedPiece)
	g.SetPiece(last.Move.To, Piece{})
	g.SetPiece(last.CapturedSquare, last.CapturedPiece)

	return nil
}

func (g *GameState) updateCastlingRights(m Move, moved Piece, captured Piece, capturedSquare Square) {
	layout := g.CastlingLayoutValue()

	switch moved.Type {
	case King:
		if moved.Color == White {
			g.CastlingRights.WhiteKingSide = false
			g.CastlingRights.WhiteQueenSide = false
		} else {
			g.CastlingRights.BlackKingSide = false
			g.CastlingRights.BlackQueenSide = false
		}
	case Rook:
		if moved.Color == White {
			if m.From == layout.RookStart(White, MoveCastleQueenSide) {
				g.CastlingRights.WhiteQueenSide = false
			}
			if m.From == layout.RookStart(White, MoveCastleKingSide) {
				g.CastlingRights.WhiteKingSide = false
			}
		} else {
			if m.From == layout.RookStart(Black, MoveCastleQueenSide) {
				g.CastlingRights.BlackQueenSide = false
			}
			if m.From == layout.RookStart(Black, MoveCastleKingSide) {
				g.CastlingRights.BlackKingSide = false
			}
		}
	}

	if captured.Type == Rook {
		if captured.Color == White {
			if capturedSquare == layout.RookStart(White, MoveCastleQueenSide) {
				g.CastlingRights.WhiteQueenSide = false
			}
			if capturedSquare == layout.RookStart(White, MoveCastleKingSide) {
				g.CastlingRights.WhiteKingSide = false
			}
		} else {
			if capturedSquare == layout.RookStart(Black, MoveCastleQueenSide) {
				g.CastlingRights.BlackQueenSide = false
			}
			if capturedSquare == layout.RookStart(Black, MoveCastleKingSide) {
				g.CastlingRights.BlackKingSide = false
			}
		}
	}
}

func enPassantCapturedSquare(to Square, mover Color) Square {
	if mover == White {
		return MustSquare(to.File(), to.Rank()-1)
	}
	return MustSquare(to.File(), to.Rank()+1)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
