package movegen

import (
	"meme_chess/internal/analyzer/position"
	"meme_chess/internal/analyzer/rules"
)

type Generator struct {
	rules rules.RuleSet
}

func NewGenerator(rs rules.RuleSet) *Generator {
	return &Generator{rules: rs}
}

func (g *Generator) GenerateLegalMoves(gs *position.GameState) []position.Move {
	pseudo := generatePseudoMoves(gs)
	legal := make([]position.Move, 0, len(pseudo))

	for _, mv := range pseudo {
		if err := g.rules.IsLegalMove(gs, mv); err == nil {
			legal = append(legal, mv)
		}
	}

	return legal
}

func generatePseudoMoves(gs *position.GameState) []position.Move {
	moves := make([]position.Move, 0, 64)

	for i := 0; i < 64; i++ {
		from := position.Square(i)
		piece := gs.PieceAt(from)
		if piece.IsZero() || piece.Color != gs.SideToMove {
			continue
		}

		switch piece.Type {
		case position.Pawn:
			moves = append(moves, genPawnMoves(gs, from, piece.Color)...)
		case position.Knight:
			moves = append(moves, genKnightMoves(gs, from)...)
		case position.Bishop:
			moves = append(moves, genSlidingMoves(gs, from, [][2]int{
				{1, 1}, {1, -1}, {-1, 1}, {-1, -1},
			})...)
		case position.Rook:
			moves = append(moves, genSlidingMoves(gs, from, [][2]int{
				{1, 0}, {-1, 0}, {0, 1}, {0, -1},
			})...)
		case position.Queen:
			moves = append(moves, genSlidingMoves(gs, from, [][2]int{
				{1, 1}, {1, -1}, {-1, 1}, {-1, -1},
				{1, 0}, {-1, 0}, {0, 1}, {0, -1},
			})...)
		case position.King:
			moves = append(moves, genKingMoves(gs, from, piece.Color)...)
		}
	}

	return moves
}

func genPawnMoves(gs *position.GameState, from position.Square, color position.Color) []position.Move {
	moves := make([]position.Move, 0, 8)

	file := from.File()
	rank := from.Rank()

	dir := 1
	startRank := 1
	promoRank := 7
	enemy := position.Black

	if color == position.Black {
		dir = -1
		startRank = 6
		promoRank = 0
		enemy = position.White
	}

	oneRank := rank + dir
	if oneRank >= 0 && oneRank <= 7 {
		to := position.MustSquare(file, oneRank)
		if gs.PieceAt(to).IsZero() {
			if oneRank == promoRank {
				moves = appendPromotionMoves(moves, from, to)
			} else {
				moves = append(moves, position.Move{From: from, To: to})
			}

			if rank == startRank {
				twoRank := rank + 2*dir
				to2 := position.MustSquare(file, twoRank)
				if gs.PieceAt(to2).IsZero() {
					moves = append(moves, position.Move{From: from, To: to2})
				}
			}
		}
	}

	for _, df := range []int{-1, 1} {
		f := file + df
		r := rank + dir
		if f < 0 || f > 7 || r < 0 || r > 7 {
			continue
		}

		to := position.MustSquare(f, r)
		dst := gs.PieceAt(to)

		if !dst.IsZero() && dst.Color == enemy {
			if r == promoRank {
				moves = appendPromotionMoves(moves, from, to)
			} else {
				moves = append(moves, position.Move{From: from, To: to})
			}
			continue
		}

		if to == gs.EnPassant {
			moves = append(moves, position.Move{
				From: from,
				To:   to,
				Kind: position.MoveEnPassant,
			})
		}
	}

	return moves
}

func appendPromotionMoves(moves []position.Move, from, to position.Square) []position.Move {
	for _, pt := range []position.PieceType{
		position.Queen,
		position.Rook,
		position.Bishop,
		position.Knight,
	} {
		moves = append(moves, position.Move{
			From:      from,
			To:        to,
			Promotion: pt,
			Kind:      position.MovePromotion,
		})
	}
	return moves
}

func genKnightMoves(gs *position.GameState, from position.Square) []position.Move {
	moves := make([]position.Move, 0, 8)
	deltas := [][2]int{
		{1, 2}, {2, 1}, {-1, 2}, {-2, 1},
		{1, -2}, {2, -1}, {-1, -2}, {-2, -1},
	}

	src := gs.PieceAt(from)

	for _, d := range deltas {
		f := from.File() + d[0]
		r := from.Rank() + d[1]
		if f < 0 || f > 7 || r < 0 || r > 7 {
			continue
		}

		to := position.MustSquare(f, r)
		dst := gs.PieceAt(to)
		if dst.IsZero() || dst.Color != src.Color {
			moves = append(moves, position.Move{From: from, To: to})
		}
	}

	return moves
}

func genSlidingMoves(gs *position.GameState, from position.Square, dirs [][2]int) []position.Move {
	moves := make([]position.Move, 0, 16)
	src := gs.PieceAt(from)

	for _, d := range dirs {
		f := from.File() + d[0]
		r := from.Rank() + d[1]

		for f >= 0 && f <= 7 && r >= 0 && r <= 7 {
			to := position.MustSquare(f, r)
			dst := gs.PieceAt(to)

			if dst.IsZero() {
				moves = append(moves, position.Move{From: from, To: to})
			} else {
				if dst.Color != src.Color {
					moves = append(moves, position.Move{From: from, To: to})
				}
				break
			}

			f += d[0]
			r += d[1]
		}
	}

	return moves
}

func genKingMoves(gs *position.GameState, from position.Square, color position.Color) []position.Move {
	moves := make([]position.Move, 0, 10)
	src := gs.PieceAt(from)

	for df := -1; df <= 1; df++ {
		for dr := -1; dr <= 1; dr++ {
			if df == 0 && dr == 0 {
				continue
			}

			f := from.File() + df
			r := from.Rank() + dr
			if f < 0 || f > 7 || r < 0 || r > 7 {
				continue
			}

			to := position.MustSquare(f, r)
			dst := gs.PieceAt(to)
			if dst.IsZero() || dst.Color != src.Color {
				moves = append(moves, position.Move{From: from, To: to})
			}
		}
	}

	layout := gs.CastlingLayoutValue()
	if color == position.White && from == layout.KingStart(position.White) {
		moves = append(moves,
			position.Move{From: from, To: layout.KingEnd(position.White, position.MoveCastleKingSide), Kind: position.MoveCastleKingSide},
			position.Move{From: from, To: layout.KingEnd(position.White, position.MoveCastleQueenSide), Kind: position.MoveCastleQueenSide},
		)
	}
	if color == position.Black && from == layout.KingStart(position.Black) {
		moves = append(moves,
			position.Move{From: from, To: layout.KingEnd(position.Black, position.MoveCastleKingSide), Kind: position.MoveCastleKingSide},
			position.Move{From: from, To: layout.KingEnd(position.Black, position.MoveCastleQueenSide), Kind: position.MoveCastleQueenSide},
		)
	}

	return moves
}
