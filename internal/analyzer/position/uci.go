package position

import (
	"fmt"
	"strings"
)

func ParseUCIMove(gs *GameState, raw string) (Move, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "o-o" || raw == "0-0" {
		if gs == nil {
			return Move{}, fmt.Errorf("castle notation requires game state")
		}
		layout := gs.CastlingLayoutValue()
		return Move{
			From: gs.findKing(layout.KingStart(gs.SideToMove), gs.SideToMove),
			To:   layout.KingEnd(gs.SideToMove, MoveCastleKingSide),
			Kind: MoveCastleKingSide,
		}, nil
	}
	if raw == "o-o-o" || raw == "0-0-0" {
		if gs == nil {
			return Move{}, fmt.Errorf("castle notation requires game state")
		}
		layout := gs.CastlingLayoutValue()
		return Move{
			From: gs.findKing(layout.KingStart(gs.SideToMove), gs.SideToMove),
			To:   layout.KingEnd(gs.SideToMove, MoveCastleQueenSide),
			Kind: MoveCastleQueenSide,
		}, nil
	}

	if len(raw) != 4 && len(raw) != 5 {
		return Move{}, fmt.Errorf("invalid uci move: %q", raw)
	}

	from, err := ParseSquare(raw[:2])
	if err != nil {
		return Move{}, fmt.Errorf("parse from square: %w", err)
	}
	to, err := ParseSquare(raw[2:4])
	if err != nil {
		return Move{}, fmt.Errorf("parse to square: %w", err)
	}

	promotion := NoPieceType
	if len(raw) == 5 {
		switch raw[4] {
		case 'q':
			promotion = Queen
		case 'r':
			promotion = Rook
		case 'b':
			promotion = Bishop
		case 'n':
			promotion = Knight
		default:
			return Move{}, fmt.Errorf("invalid promotion: %q", string(raw[4]))
		}
	}

	move := Move{
		From:      from,
		To:        to,
		Promotion: promotion,
		Kind:      MoveNormal,
	}
	if promotion != NoPieceType {
		move.Kind = MovePromotion
	}

	if gs == nil {
		return move, nil
	}

	piece := gs.PieceAt(from)
	if piece.IsZero() {
		return Move{}, fmt.Errorf("no piece at source square %s", from)
	}

	layout := gs.CastlingLayoutValue()
	if piece.Type == King && from == layout.KingStart(piece.Color) {
		if to == layout.KingEnd(piece.Color, MoveCastleKingSide) {
			move.Kind = MoveCastleKingSide
			move.Promotion = NoPieceType
		}
		if to == layout.KingEnd(piece.Color, MoveCastleQueenSide) {
			move.Kind = MoveCastleQueenSide
			move.Promotion = NoPieceType
		}
	}

	if piece.Type == Pawn && move.Kind != MovePromotion && from.File() != to.File() && gs.PieceAt(to).IsZero() && gs.EnPassant == to {
		move.Kind = MoveEnPassant
	}

	return move, nil
}

func (g *GameState) findKing(expected Square, color Color) Square {
	if piece := g.PieceAt(expected); !piece.IsZero() && piece.Type == King && piece.Color == color {
		return expected
	}

	for i := 0; i < 64; i++ {
		sq := Square(i)
		piece := g.PieceAt(sq)
		if !piece.IsZero() && piece.Type == King && piece.Color == color {
			return sq
		}
	}

	return expected
}

func BuildGameStateFromUCIMoves(moves []string) (*GameState, error) {
	gs := NewInitial()
	for _, raw := range moves {
		mv, err := ParseUCIMove(gs, raw)
		if err != nil {
			return nil, err
		}
		if err := gs.ApplyMove(mv); err != nil {
			return nil, err
		}
	}
	return gs, nil
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
