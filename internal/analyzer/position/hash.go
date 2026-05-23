package position

import (
	"crypto/sha256"
	"encoding/hex"
)

var (
	zobristPiece  [2][7][64]uint64
	zobristCastle [16]uint64
	zobristEPFile [9]uint64
	zobristSide   uint64
)

func init() {
	var seed uint64 = 0x9e3779b97f4a7c15

	next := func() uint64 {
		seed += 0x9e3779b97f4a7c15
		z := seed
		z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
		z = (z ^ (z >> 27)) * 0x94d049bb133111eb
		return z ^ (z >> 31)
	}

	for color := range zobristPiece {
		for pieceType := range zobristPiece[color] {
			for square := range zobristPiece[color][pieceType] {
				zobristPiece[color][pieceType][square] = next()
			}
		}
	}

	for i := range zobristCastle {
		zobristCastle[i] = next()
	}
	for i := range zobristEPFile {
		zobristEPFile[i] = next()
	}
	zobristSide = next()
}

func (g *GameState) Key() uint64 {
	var key uint64

	for square, piece := range g.Board {
		if piece.IsZero() {
			continue
		}
		key ^= zobristPiece[piece.Color][piece.Type][square]
	}

	if g.SideToMove == Black {
		key ^= zobristSide
	}

	var castleIndex uint8
	if g.CastlingRights.WhiteKingSide {
		castleIndex |= 1
	}
	if g.CastlingRights.WhiteQueenSide {
		castleIndex |= 2
	}
	if g.CastlingRights.BlackKingSide {
		castleIndex |= 4
	}
	if g.CastlingRights.BlackQueenSide {
		castleIndex |= 8
	}
	key ^= zobristCastle[castleIndex]

	epIndex := 8
	if g.EnPassant != NoSquare {
		epIndex = g.EnPassant.File()
	}
	key ^= zobristEPFile[epIndex]

	return key
}

func (g *GameState) Hash() string {
	return HashFEN(g.FEN())
}

func HashFEN(fen string) string {
	sum := sha256.Sum256([]byte(fen))
	return hex.EncodeToString(sum[:])
}
