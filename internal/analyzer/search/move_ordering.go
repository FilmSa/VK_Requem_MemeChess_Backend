package search

import (
	"meme_chess/internal/analyzer/position"
	"meme_chess/internal/analyzer/rules"
	"sort"
)

type MoveOrdering interface {
	Order(gs *position.GameState, moves []position.Move, ttMove position.Move) []position.Move
}

type DefaultMoveOrdering struct{}

func NewMoveOrdering(rs rules.RuleSet) MoveOrdering {
	_ = rs
	return DefaultMoveOrdering{}
}

type scoredMove struct {
	move  position.Move
	score int
}

func (e *Engine) orderMoves(gs *position.GameState, moves []position.Move, ttMove position.Move, ply int) []position.Move {
	scored := make([]scoredMove, 0, len(moves))
	_ = ply

	for _, mv := range moves {
		scored = append(scored, scoredMove{
			move:  mv,
			score: movePriority(gs, mv, ttMove, nil),
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	ordered := make([]position.Move, len(scored))
	for i := range scored {
		ordered[i] = scored[i].move
	}

	return ordered
}

func (o DefaultMoveOrdering) Order(gs *position.GameState, moves []position.Move, ttMove position.Move) []position.Move {
	scored := make([]scoredMove, 0, len(moves))
	for _, mv := range moves {
		scored = append(scored, scoredMove{
			move:  mv,
			score: movePriority(gs, mv, ttMove, nil),
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	ordered := make([]position.Move, len(scored))
	for i := range scored {
		ordered[i] = scored[i].move
	}

	return ordered
}

func movePriority(gs *position.GameState, mv position.Move, ttMove position.Move, _ rules.RuleSet) int {
	score := 0

	if sameMove(mv, ttMove) {
		score += 50000
	}

	if captured := capturedPieceForMove(gs, mv); !captured.IsZero() {
		score += 10000 + pieceValue(captured.Type)*10 - pieceValue(gs.PieceAt(mv.From).Type)
	}

	switch mv.Kind {
	case position.MovePromotion:
		score += 9000 + pieceValue(mv.Promotion)
	case position.MoveEnPassant:
		score += 7000
	case position.MoveCastleKingSide, position.MoveCastleQueenSide:
		score += 500
	}

	// Mild centralization bonus helps move ordering without changing evaluation.
	to := mv.To
	score += 14 - abs(3-to.File()) - abs(3-to.Rank())

	return score
}

func capturedPieceForMove(gs *position.GameState, mv position.Move) position.Piece {
	switch mv.Kind {
	case position.MoveEnPassant:
		mover := gs.PieceAt(mv.From)
		if mover.Color == position.White {
			return gs.PieceAt(position.MustSquare(mv.To.File(), mv.To.Rank()-1))
		}
		return gs.PieceAt(position.MustSquare(mv.To.File(), mv.To.Rank()+1))
	case position.MoveCastleKingSide, position.MoveCastleQueenSide:
		return position.Piece{}
	default:
		return gs.PieceAt(mv.To)
	}
}
