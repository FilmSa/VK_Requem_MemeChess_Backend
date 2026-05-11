package game

import "meme_chess/internal/analyzer/position"

const (
	EffectTypePawnCounter  = "pawn_counter"
	EffectTypeKingRevenge  = "king_revenge"
	EffectTypeKnightDouble = "knight_double"
	EffectTypeBishopPierce = "bishop_pierce"
	EffectTypeRookRampage  = "rook_rampage"
)

type MoveEffect struct {
	Type      string          `json:"type"`
	Title     string          `json:"title,omitempty"`
	Message   string          `json:"message,omitempty"`
	Piece     string          `json:"piece,omitempty"`
	Color     string          `json:"color,omitempty"`
	From      string          `json:"from,omitempty"`
	To        string          `json:"to,omitempty"`
	Removed   []AffectedPiece `json:"removed,omitempty"`
	Animation *AnimationHint  `json:"animation,omitempty"`
}

type AffectedPiece struct {
	Square     string `json:"square"`
	Piece      string `json:"piece"`
	Color      string `json:"color"`
	Relation   string `json:"relation,omitempty"`
	KnockbackX int    `json:"knockback_x,omitempty"`
	KnockbackY int    `json:"knockback_y,omitempty"`
}

type AnimationHint struct {
	Name       string `json:"name"`
	DurationMs int    `json:"duration_ms"`
	Easing     string `json:"easing,omitempty"`
}

func cloneEffects(in []MoveEffect) []MoveEffect {
	if len(in) == 0 {
		return nil
	}

	out := make([]MoveEffect, len(in))
	for i := range in {
		out[i] = in[i]
		if len(in[i].Removed) > 0 {
			out[i].Removed = append([]AffectedPiece(nil), in[i].Removed...)
		}
		if in[i].Animation != nil {
			anim := *in[i].Animation
			out[i].Animation = &anim
		}
	}
	return out
}

func pieceTypeName(pieceType position.PieceType) string {
	switch pieceType {
	case position.Pawn:
		return "pawn"
	case position.Knight:
		return "knight"
	case position.Bishop:
		return "bishop"
	case position.Rook:
		return "rook"
	case position.Queen:
		return "queen"
	case position.King:
		return "king"
	default:
		return ""
	}
}

func colorName(color position.Color) string {
	if color == position.White {
		return "white"
	}
	return "black"
}

func affectedPiece(square position.Square, piece position.Piece, relation string, knockbackX int, knockbackY int) AffectedPiece {
	return AffectedPiece{
		Square:     square.String(),
		Piece:      pieceTypeName(piece.Type),
		Color:      colorName(piece.Color),
		Relation:   relation,
		KnockbackX: knockbackX,
		KnockbackY: knockbackY,
	}
}

func rookKnockback(index int, from position.Square, to position.Square) (int, int) {
	pattern := [][2]int{
		{-1, -2},
		{1, -2},
		{-2, -1},
		{2, -1},
		{-2, 1},
		{2, 1},
		{-1, 2},
		{1, 2},
	}

	step := pattern[index%len(pattern)]
	scale := 28 + ((index % 3) * 10)

	if from.File() == to.File() {
		return step[0] * scale, step[1] * scale
	}
	if from.Rank() == to.Rank() {
		return step[1] * scale, step[0] * scale
	}
	return step[0] * scale, step[1] * scale
}
