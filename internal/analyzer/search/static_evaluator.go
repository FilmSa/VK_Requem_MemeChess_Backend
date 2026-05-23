package search

import "meme_chess/internal/analyzer/position"

type StaticEvaluator interface {
	Evaluate(gs *position.GameState) int
}

type defaultStaticEvaluator struct{}

var (
	searchPawnTable = [64]int{
		0, 0, 0, 0, 0, 0, 0, 0,
		48, 48, 48, 48, 48, 48, 48, 48,
		8, 10, 16, 24, 24, 16, 10, 8,
		4, 6, 10, 18, 18, 10, 6, 4,
		2, 4, 8, 14, 14, 8, 4, 2,
		2, 0, -4, 0, 0, -4, 0, 2,
		2, 4, 4, -12, -12, 4, 4, 2,
		0, 0, 0, 0, 0, 0, 0, 0,
	}
	searchKnightTable = [64]int{
		-40, -28, -20, -16, -16, -20, -28, -40,
		-28, -12, 0, 4, 4, 0, -12, -28,
		-20, 4, 10, 14, 14, 10, 4, -20,
		-16, 6, 14, 18, 18, 14, 6, -16,
		-16, 4, 14, 18, 18, 14, 4, -16,
		-20, 6, 10, 14, 14, 10, 6, -20,
		-28, -12, 0, 6, 6, 0, -12, -28,
		-40, -28, -20, -16, -16, -20, -28, -40,
	}
	searchBishopTable = [64]int{
		-16, -10, -8, -8, -8, -8, -10, -16,
		-10, 2, 0, 0, 0, 0, 2, -10,
		-8, 6, 8, 10, 10, 8, 6, -8,
		-8, 0, 10, 12, 12, 10, 0, -8,
		-8, 4, 10, 12, 12, 10, 4, -8,
		-8, 8, 10, 10, 10, 10, 8, -8,
		-10, 4, 0, 0, 0, 0, 4, -10,
		-16, -10, -8, -8, -8, -8, -10, -16,
	}
	searchRookTable = [64]int{
		0, 0, 4, 8, 8, 4, 0, 0,
		-2, 0, 0, 0, 0, 0, 0, -2,
		-2, 0, 0, 0, 0, 0, 0, -2,
		-2, 0, 0, 0, 0, 0, 0, -2,
		-2, 0, 0, 0, 0, 0, 0, -2,
		-2, 0, 0, 0, 0, 0, 0, -2,
		4, 8, 8, 8, 8, 8, 8, 4,
		0, 0, 4, 8, 8, 4, 0, 0,
	}
	searchQueenTable = [64]int{
		-16, -8, -8, -4, -4, -8, -8, -16,
		-8, 0, 2, 0, 0, 0, 0, -8,
		-8, 2, 6, 6, 6, 6, 0, -8,
		-4, 0, 6, 6, 6, 6, 0, -4,
		-4, 0, 6, 6, 6, 6, 0, -4,
		-8, 0, 6, 6, 6, 6, 0, -8,
		-8, 0, 0, 0, 0, 0, 0, -8,
		-16, -8, -8, -4, -4, -8, -8, -16,
	}
	searchKingMidgameTable = [64]int{
		-36, -36, -36, -44, -44, -36, -36, -36,
		-28, -28, -28, -36, -36, -28, -28, -28,
		-18, -18, -18, -28, -28, -18, -18, -18,
		-8, -8, -8, -18, -18, -8, -8, -8,
		0, 0, -8, -18, -18, -8, 0, 0,
		8, 8, 0, -8, -8, 0, 8, 8,
		20, 20, 8, 0, 0, 8, 20, 20,
		20, 28, 12, 0, 0, 12, 28, 20,
	}
	searchKingEndgameTable = [64]int{
		-32, -24, -16, -8, -8, -16, -24, -32,
		-24, -8, 0, 0, 0, 0, -8, -24,
		-16, 0, 8, 12, 12, 8, 0, -16,
		-8, 0, 12, 18, 18, 12, 0, -8,
		-8, 0, 12, 18, 18, 12, 0, -8,
		-16, 0, 8, 12, 12, 8, 0, -16,
		-24, -8, 0, 0, 0, 0, -8, -24,
		-32, -24, -16, -8, -8, -16, -24, -32,
	}
)

func NewStaticEvaluator() StaticEvaluator {
	return defaultStaticEvaluator{}
}

func (defaultStaticEvaluator) Evaluate(gs *position.GameState) int {
	score := 0
	queens := 0
	nonPawnMaterial := 0
	bishops := [2]int{}

	for i := 0; i < 64; i++ {
		piece := gs.PieceAt(position.Square(i))
		if piece.IsZero() {
			continue
		}
		if piece.Type == position.Queen {
			queens++
		}
		if piece.Type != position.Pawn && piece.Type != position.King {
			nonPawnMaterial += pieceValue(piece.Type)
		}
		if piece.Type == position.Bishop {
			bishops[colorIndex(piece.Color)]++
		}
	}

	endgame := queens == 0 || nonPawnMaterial <= 2600

	for i := 0; i < 64; i++ {
		sq := position.Square(i)
		piece := gs.PieceAt(sq)
		if piece.IsZero() {
			continue
		}

		value := pieceValue(piece.Type) + pieceSquareValue(piece, sq, endgame)
		if piece.Color == position.White {
			score += value
		} else {
			score -= value
		}
	}

	if bishops[colorIndex(position.White)] >= 2 {
		score += 28
	}
	if bishops[colorIndex(position.Black)] >= 2 {
		score -= 28
	}

	if gs.SideToMove == position.Black {
		return -score
	}
	return score
}

func pieceSquareValue(piece position.Piece, sq position.Square, endgame bool) int {
	index := tableIndex(sq, piece.Color)

	switch piece.Type {
	case position.Pawn:
		return searchPawnTable[index]
	case position.Knight:
		return searchKnightTable[index]
	case position.Bishop:
		return searchBishopTable[index]
	case position.Rook:
		return searchRookTable[index]
	case position.Queen:
		return searchQueenTable[index]
	case position.King:
		if endgame {
			return searchKingEndgameTable[index]
		}
		return searchKingMidgameTable[index]
	default:
		return 0
	}
}

func tableIndex(sq position.Square, color position.Color) int {
	if color == position.White {
		return int(sq)
	}
	return (7-sq.Rank())*8 + sq.File()
}

func colorIndex(color position.Color) int {
	if color == position.White {
		return 0
	}
	return 1
}
