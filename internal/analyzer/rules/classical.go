package rules

import (
	"meme_chess/internal/analyzer/position"
)

type ClassicalRuleSet struct{}

func NewClassicalRuleSet() *ClassicalRuleSet {
	return &ClassicalRuleSet{}
}

func (r *ClassicalRuleSet) IsLegalMove(gs *position.GameState, mv position.Move) error {
	piece := gs.PieceAt(mv.From)
	if piece.IsZero() {
		return ErrNoPieceAtSource
	}

	if piece.Color != gs.SideToMove {
		return ErrWrongSideToMove
	}

	dst := gs.PieceAt(mv.To)
	if !dst.IsZero() && dst.Color == piece.Color && !(piece.Type == position.King && (mv.Kind == position.MoveCastleKingSide || mv.Kind == position.MoveCastleQueenSide)) {
		return ErrDestinationOccupied
	}

	if err := r.validatePromotion(gs, mv, piece); err != nil {
		return err
	}

	if !r.isPseudoLegal(gs, mv, piece) {
		return ErrIllegalGeometry
	}

	clone := gs.Clone()
	if err := clone.ApplyMove(mv); err != nil {
		return err
	}

	if r.IsCheck(clone, piece.Color) {
		return ErrKingLeftInCheck
	}

	if piece.Type == position.King && (mv.Kind == position.MoveCastleKingSide || mv.Kind == position.MoveCastleQueenSide) {
		if err := r.validateCastlePath(gs, mv, piece.Color); err != nil {
			return err
		}
	}

	return nil
}

func (r *ClassicalRuleSet) IsCheck(gs *position.GameState, color position.Color) bool {
	kingSq, ok := findKing(gs, color)
	if !ok {
		return false
	}

	return squareAttackedBy(gs, kingSq, color.Opponent())
}

func (r *ClassicalRuleSet) isPseudoLegal(gs *position.GameState, mv position.Move, piece position.Piece) bool {
	fromFile, fromRank := mv.From.File(), mv.From.Rank()
	toFile, toRank := mv.To.File(), mv.To.Rank()

	df := toFile - fromFile
	dr := toRank - fromRank

	switch piece.Type {
	case position.Pawn:
		return isLegalPawnMove(gs, mv, piece.Color)
	case position.Knight:
		return (abs(df) == 1 && abs(dr) == 2) || (abs(df) == 2 && abs(dr) == 1)
	case position.Bishop:
		return abs(df) == abs(dr) && isPathClear(gs, mv.From, mv.To)
	case position.Rook:
		return (df == 0 || dr == 0) && isPathClear(gs, mv.From, mv.To)
	case position.Queen:
		return (df == 0 || dr == 0 || abs(df) == abs(dr)) && isPathClear(gs, mv.From, mv.To)
	case position.King:
		if mv.Kind == position.MoveCastleKingSide || mv.Kind == position.MoveCastleQueenSide {
			return r.canCastleGeometry(gs, mv, piece.Color)
		}
		return abs(df) <= 1 && abs(dr) <= 1
	default:
		return false
	}
}

func (r *ClassicalRuleSet) validatePromotion(gs *position.GameState, mv position.Move, piece position.Piece) error {
	if piece.Type != position.Pawn {
		if mv.Promotion != position.NoPieceType {
			return ErrInvalidPromotion
		}
		return nil
	}

	lastRank := 7
	if piece.Color == position.Black {
		lastRank = 0
	}

	reachesLastRank := mv.To.Rank() == lastRank
	if reachesLastRank && mv.Promotion == position.NoPieceType {
		return ErrInvalidPromotion
	}
	if !reachesLastRank && mv.Promotion != position.NoPieceType {
		return ErrInvalidPromotion
	}
	if mv.Promotion != position.NoPieceType {
		switch mv.Promotion {
		case position.Queen, position.Rook, position.Bishop, position.Knight:
			return nil
		default:
			return ErrInvalidPromotion
		}
	}
	return nil
}

func (r *ClassicalRuleSet) canCastleGeometry(gs *position.GameState, mv position.Move, color position.Color) bool {
	layout := gs.CastlingLayoutValue()
	if mv.From != layout.KingStart(color) || mv.To != layout.KingEnd(color, mv.Kind) {
		return false
	}

	if color == position.White {
		if mv.Kind == position.MoveCastleKingSide && !gs.CastlingRights.WhiteKingSide {
			return false
		}
		if mv.Kind == position.MoveCastleQueenSide && !gs.CastlingRights.WhiteQueenSide {
			return false
		}
	} else {
		if mv.Kind == position.MoveCastleKingSide && !gs.CastlingRights.BlackKingSide {
			return false
		}
		if mv.Kind == position.MoveCastleQueenSide && !gs.CastlingRights.BlackQueenSide {
			return false
		}
	}

	rookStart := layout.RookStart(color, mv.Kind)
	rook := gs.PieceAt(rookStart)
	if rook.Type != position.Rook || rook.Color != color {
		return false
	}

	ignore := map[position.Square]struct{}{
		layout.KingStart(color): {},
		rookStart:               {},
	}

	for _, sq := range layout.KingPath(color, mv.Kind)[1:] {
		if _, ok := ignore[sq]; ok {
			continue
		}
		if !gs.PieceAt(sq).IsZero() {
			return false
		}
	}

	for _, sq := range layout.RookPath(color, mv.Kind)[1:] {
		if _, ok := ignore[sq]; ok {
			continue
		}
		if !gs.PieceAt(sq).IsZero() {
			return false
		}
	}

	return true
}

func (r *ClassicalRuleSet) validateCastlePath(gs *position.GameState, mv position.Move, color position.Color) error {
	if r.IsCheck(gs, color) {
		return ErrIllegalCastle
	}

	layout := gs.CastlingLayoutValue()
	rookStart := layout.RookStart(color, mv.Kind)
	for _, sq := range layout.KingPath(color, mv.Kind)[1:] {
		tmp := gs.Clone()
		king := tmp.PieceAt(mv.From)
		tmp.SetPiece(mv.From, position.Piece{})
		tmp.SetPiece(rookStart, position.Piece{})
		tmp.SetPiece(sq, king)
		if r.IsCheck(tmp, color) {
			return ErrIllegalCastle
		}
	}

	return nil
}

func findKing(gs *position.GameState, color position.Color) (position.Square, bool) {
	return gs.KingSquare(color)
}

func squareAttackedBy(gs *position.GameState, target position.Square, attacker position.Color) bool {
	file := target.File()
	rank := target.Rank()

	pawnRank := rank - 1
	if attacker == position.Black {
		pawnRank = rank + 1
	}
	if pawnRank >= 0 && pawnRank <= 7 {
		for _, df := range [2]int{-1, 1} {
			f := file + df
			if f < 0 || f > 7 {
				continue
			}
			sq := position.MustSquare(f, pawnRank)
			piece := gs.PieceAt(sq)
			if piece.Color == attacker && piece.Type == position.Pawn {
				return true
			}
		}
	}

	for _, d := range [][2]int{
		{1, 2}, {2, 1}, {-1, 2}, {-2, 1},
		{1, -2}, {2, -1}, {-1, -2}, {-2, -1},
	} {
		f := file + d[0]
		r := rank + d[1]
		if f < 0 || f > 7 || r < 0 || r > 7 {
			continue
		}
		piece := gs.PieceAt(position.MustSquare(f, r))
		if piece.Color == attacker && piece.Type == position.Knight {
			return true
		}
	}

	for _, d := range [][2]int{
		{1, 0}, {-1, 0}, {0, 1}, {0, -1},
		{1, 1}, {1, -1}, {-1, 1}, {-1, -1},
	} {
		f := file + d[0]
		r := rank + d[1]
		if f < 0 || f > 7 || r < 0 || r > 7 {
			continue
		}
		piece := gs.PieceAt(position.MustSquare(f, r))
		if piece.Color == attacker && piece.Type == position.King {
			return true
		}
	}

	for _, d := range [][2]int{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
		for f, r := file+d[0], rank+d[1]; f >= 0 && f <= 7 && r >= 0 && r <= 7; f, r = f+d[0], r+d[1] {
			piece := gs.PieceAt(position.MustSquare(f, r))
			if piece.IsZero() {
				continue
			}
			return piece.Color == attacker && (piece.Type == position.Bishop || piece.Type == position.Queen)
		}
	}

	for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		for f, r := file+d[0], rank+d[1]; f >= 0 && f <= 7 && r >= 0 && r <= 7; f, r = f+d[0], r+d[1] {
			piece := gs.PieceAt(position.MustSquare(f, r))
			if piece.IsZero() {
				continue
			}
			return piece.Color == attacker && (piece.Type == position.Rook || piece.Type == position.Queen)
		}
	}

	return false
}

func AttacksSquare(gs *position.GameState, from, to position.Square, piece position.Piece) bool {
	df := to.File() - from.File()
	dr := to.Rank() - from.Rank()

	switch piece.Type {
	case position.Pawn:
		if piece.Color == position.White {
			return dr == 1 && (df == 1 || df == -1)
		}
		return dr == -1 && (df == 1 || df == -1)
	case position.Knight:
		return (abs(df) == 1 && abs(dr) == 2) || (abs(df) == 2 && abs(dr) == 1)
	case position.Bishop:
		return abs(df) == abs(dr) && isPathClear(gs, from, to)
	case position.Rook:
		return (df == 0 || dr == 0) && isPathClear(gs, from, to)
	case position.Queen:
		return (df == 0 || dr == 0 || abs(df) == abs(dr)) && isPathClear(gs, from, to)
	case position.King:
		return abs(df) <= 1 && abs(dr) <= 1
	default:
		return false
	}
}

func isLegalPawnMove(gs *position.GameState, mv position.Move, color position.Color) bool {
	fromFile, fromRank := mv.From.File(), mv.From.Rank()
	toFile, toRank := mv.To.File(), mv.To.Rank()

	df := toFile - fromFile
	dr := toRank - fromRank
	dst := gs.PieceAt(mv.To)

	if color == position.White {
		if df == 0 && dr == 1 && dst.IsZero() {
			return true
		}
		if df == 0 && dr == 2 && fromRank == 1 && dst.IsZero() {
			mid := position.MustSquare(fromFile, fromRank+1)
			return gs.PieceAt(mid).IsZero()
		}
		if abs(df) == 1 && dr == 1 && !dst.IsZero() && dst.Color == position.Black {
			return true
		}
		if abs(df) == 1 && dr == 1 && mv.To == gs.EnPassant {
			capSq := position.MustSquare(toFile, toRank-1)
			p := gs.PieceAt(capSq)
			return p.Type == position.Pawn && p.Color == position.Black
		}
		return false
	}

	if df == 0 && dr == -1 && dst.IsZero() {
		return true
	}
	if df == 0 && dr == -2 && fromRank == 6 && dst.IsZero() {
		mid := position.MustSquare(fromFile, fromRank-1)
		return gs.PieceAt(mid).IsZero()
	}
	if abs(df) == 1 && dr == -1 && !dst.IsZero() && dst.Color == position.White {
		return true
	}
	if abs(df) == 1 && dr == -1 && mv.To == gs.EnPassant {
		capSq := position.MustSquare(toFile, toRank+1)
		p := gs.PieceAt(capSq)
		return p.Type == position.Pawn && p.Color == position.White
	}
	return false
}

func isPathClear(gs *position.GameState, from, to position.Square) bool {
	df := sign(to.File() - from.File())
	dr := sign(to.Rank() - from.Rank())

	f := from.File() + df
	r := from.Rank() + dr

	for f != to.File() || r != to.Rank() {
		sq := position.MustSquare(f, r)
		if !gs.PieceAt(sq).IsZero() {
			return false
		}
		f += df
		r += dr
	}

	return true
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func sign(x int) int {
	if x < 0 {
		return -1
	}
	if x > 0 {
		return 1
	}
	return 0
}
