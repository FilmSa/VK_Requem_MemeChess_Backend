package rules

import "meme_chess/internal/analyzer/position"

type EvolutionRuleSet struct {
	*ClassicalRuleSet
	RookRampage  bool
	BishopPierce bool
}

func NewEvolutionRuleSet(rookRampage bool, bishopPierce bool) *EvolutionRuleSet {
	return &EvolutionRuleSet{
		ClassicalRuleSet: NewClassicalRuleSet(),
		RookRampage:      rookRampage,
		BishopPierce:     bishopPierce,
	}
}

func (r *EvolutionRuleSet) IsLegalMove(gs *position.GameState, mv position.Move) error {
	if !r.RookRampage && !r.BishopPierce {
		return r.ClassicalRuleSet.IsLegalMove(gs, mv)
	}

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

func (r *EvolutionRuleSet) IsCheck(gs *position.GameState, color position.Color) bool {
	kingSq, ok := findKing(gs, color)
	if !ok {
		return false
	}

	enemy := color.Opponent()
	for i := 0; i < 64; i++ {
		from := position.Square(i)
		piece := gs.PieceAt(from)
		if piece.IsZero() || piece.Color != enemy {
			continue
		}

		if r.attacksSquare(gs, from, kingSq, piece) {
			return true
		}
	}

	return false
}

func (r *EvolutionRuleSet) isPseudoLegal(gs *position.GameState, mv position.Move, piece position.Piece) bool {
	if piece.Type == position.Bishop && r.BishopPierce {
		df := mv.To.File() - mv.From.File()
		dr := mv.To.Rank() - mv.From.Rank()
		return abs(df) == abs(dr) && isBishopPiercePathClear(gs, mv.From, mv.To)
	}

	if piece.Type != position.Rook {
		return r.ClassicalRuleSet.isPseudoLegal(gs, mv, piece)
	}

	if !r.RookRampage {
		return r.ClassicalRuleSet.isPseudoLegal(gs, mv, piece)
	}

	df := mv.To.File() - mv.From.File()
	dr := mv.To.Rank() - mv.From.Rank()
	if (df != 0 && dr != 0) || (df == 0 && dr == 0) {
		return false
	}

	return !containsKingOnPath(gs, mv.From, mv.To, true)
}

func (r *EvolutionRuleSet) attacksSquare(gs *position.GameState, from, to position.Square, piece position.Piece) bool {
	if piece.Type == position.Bishop && r.BishopPierce {
		df := to.File() - from.File()
		dr := to.Rank() - from.Rank()
		return abs(df) == abs(dr) && isBishopPiercePathClear(gs, from, to)
	}

	if piece.Type != position.Rook || !r.RookRampage {
		return AttacksSquare(gs, from, to, piece)
	}

	df := to.File() - from.File()
	dr := to.Rank() - from.Rank()
	if (df != 0 && dr != 0) || (df == 0 && dr == 0) {
		return false
	}

	return !containsKingOnPath(gs, from, to, false)
}

func containsKingOnPath(gs *position.GameState, from, to position.Square, includeDestination bool) bool {
	df := sign(to.File() - from.File())
	dr := sign(to.Rank() - from.Rank())

	for f, rank := from.File()+df, from.Rank()+dr; f != to.File() || rank != to.Rank(); f, rank = f+df, rank+dr {
		sq := position.MustSquare(f, rank)
		if gs.PieceAt(sq).Type == position.King {
			return true
		}
	}

	if includeDestination && gs.PieceAt(to).Type == position.King {
		return true
	}

	return false
}

func isBishopPiercePathClear(gs *position.GameState, from, to position.Square) bool {
	df := sign(to.File() - from.File())
	dr := sign(to.Rank() - from.Rank())

	f := from.File() + df
	r := from.Rank() + dr
	for f != to.File() || r != to.Rank() {
		sq := position.MustSquare(f, r)
		piece := gs.PieceAt(sq)
		if !piece.IsZero() && piece.Type != position.Pawn {
			return false
		}
		f += df
		r += dr
	}

	return true
}
