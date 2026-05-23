package search

import (
	"meme_chess/internal/analyzer/position"
	"meme_chess/internal/analyzer/rules"
)

func pieceValue(pt position.PieceType) int {
	switch pt {
	case position.Pawn:
		return 100
	case position.Knight:
		return 320
	case position.Bishop:
		return 330
	case position.Rook:
		return 500
	case position.Queen:
		return 900
	default:
		return 0
	}
}

func sameMove(a, b position.Move) bool {
	return a.From == b.From && a.To == b.To && a.Kind == b.Kind && a.Promotion == b.Promotion
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func isTacticalMove(gs *position.GameState, mv position.Move) bool {
	if mv.Kind == position.MovePromotion || mv.Kind == position.MoveEnPassant {
		return true
	}
	return !capturedPieceForMove(gs, mv).IsZero()
}

func attackers(gs *position.GameState, target position.Square, color position.Color) []position.Square {
	out := make([]position.Square, 0, 4)
	for i := 0; i < 64; i++ {
		from := position.Square(i)
		p := gs.PieceAt(from)
		if p.IsZero() || p.Color != color {
			continue
		}
		if rules.AttacksSquare(gs, from, target, p) {
			out = append(out, from)
		}
	}
	return out
}

func lowestAttackerValue(gs *position.GameState, squares []position.Square) int {
	best := 1 << 30
	for _, sq := range squares {
		value := pieceValue(gs.PieceAt(sq).Type)
		if value < best {
			best = value
		}
	}
	if best == 1<<30 {
		return 0
	}
	return best
}

func isWeakTarget(gs *position.GameState, sq position.Square, color position.Color) bool {
	piece := gs.PieceAt(sq)
	if piece.IsZero() || piece.Color != color || piece.Type == position.King {
		return false
	}

	enemyAttackers := attackers(gs, sq, color.Opponent())
	if len(enemyAttackers) == 0 {
		return false
	}

	defenders := attackers(gs, sq, color)
	return len(defenders) == 0 || lowestAttackerValue(gs, enemyAttackers) < pieceValue(piece.Type)
}
