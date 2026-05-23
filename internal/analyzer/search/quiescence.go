package search

import (
	"meme_chess/internal/analyzer/position"
)

const maxQuiescenceDepth = 4

func (e *Engine) quiescence(gs *position.GameState, ply int, alpha, beta int, nodes *int) int {
	*nodes = *nodes + 1
	standPat := e.static.Evaluate(gs)
	if standPat >= beta {
		return beta
	}
	if standPat > alpha {
		alpha = standPat
	}

	if ply >= maxQuiescenceDepth {
		return standPat
	}

	moves := e.noisyMoves(gs)
	if len(moves) == 0 {
		return standPat
	}

	ordered := e.orderMoves(gs, moves, position.NullMove(), ply)
	for _, mv := range ordered {
		if err := gs.ApplyMove(mv); err != nil {
			continue
		}

		score := -e.quiescence(gs, ply+1, -beta, -alpha, nodes)

		if err := gs.UndoMove(); err != nil {
			panic(err)
		}

		if score >= beta {
			return beta
		}
		if score > alpha {
			alpha = score
		}
	}

	return alpha
}

func (e *Engine) noisyMoves(gs *position.GameState) []position.Move {
	pseudo := e.gen.GeneratePseudoMoves(gs)
	out := make([]position.Move, 0, len(pseudo))
	movingSide := gs.SideToMove

	for _, mv := range pseudo {
		if mv.Kind != position.MovePromotion &&
			mv.Kind != position.MoveEnPassant &&
			capturedPieceForMove(gs, mv).IsZero() {
			continue
		}
		if err := gs.ApplyMove(mv); err != nil {
			continue
		}

		legal := !e.rules.IsCheck(gs, movingSide)
		if err := gs.UndoMove(); err != nil {
			panic(err)
		}

		if legal {
			out = append(out, mv)
		}
	}

	return out
}

func (e *Engine) isNoisyMove(gs *position.GameState, mv position.Move) bool {
	if mv.Kind == position.MovePromotion || mv.Kind == position.MoveEnPassant {
		return true
	}
	if !capturedPieceForMove(gs, mv).IsZero() {
		return true
	}

	if err := gs.ApplyMove(mv); err != nil {
		return false
	}
	inCheck := e.rules.IsCheck(gs, gs.SideToMove)
	if err := gs.UndoMove(); err != nil {
		panic(err)
	}

	// Keep forcing continuations alive inside q-search.
	return inCheck
}
