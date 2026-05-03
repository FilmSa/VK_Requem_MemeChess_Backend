package position

type CastlingSideLayout struct {
	KingStart          Square
	KingSideRookStart  Square
	QueenSideRookStart Square
}

type CastlingLayout struct {
	White CastlingSideLayout
	Black CastlingSideLayout
}

func StandardCastlingLayout() *CastlingLayout {
	return &CastlingLayout{
		White: CastlingSideLayout{
			KingStart:          MustSquare(4, 0),
			KingSideRookStart:  MustSquare(7, 0),
			QueenSideRookStart: MustSquare(0, 0),
		},
		Black: CastlingSideLayout{
			KingStart:          MustSquare(4, 7),
			KingSideRookStart:  MustSquare(7, 7),
			QueenSideRookStart: MustSquare(0, 7),
		},
	}
}

func (g *GameState) CastlingLayoutValue() CastlingLayout {
	if g != nil && g.CastlingLayout != nil {
		return *g.CastlingLayout
	}
	return *StandardCastlingLayout()
}

func (l CastlingLayout) side(color Color) CastlingSideLayout {
	if color == White {
		return l.White
	}
	return l.Black
}

func (l CastlingLayout) KingStart(color Color) Square {
	return l.side(color).KingStart
}

func (l CastlingLayout) RookStart(color Color, kind MoveKind) Square {
	side := l.side(color)
	if kind == MoveCastleKingSide {
		return side.KingSideRookStart
	}
	return side.QueenSideRookStart
}

func (l CastlingLayout) KingEnd(color Color, kind MoveKind) Square {
	rank := 0
	if color == Black {
		rank = 7
	}
	if kind == MoveCastleKingSide {
		return MustSquare(6, rank)
	}
	return MustSquare(2, rank)
}

func (l CastlingLayout) RookEnd(color Color, kind MoveKind) Square {
	rank := 0
	if color == Black {
		rank = 7
	}
	if kind == MoveCastleKingSide {
		return MustSquare(5, rank)
	}
	return MustSquare(3, rank)
}

func rankPath(from, to Square) []Square {
	if from == to {
		return []Square{from}
	}

	step := 1
	if to.File() < from.File() {
		step = -1
	}

	path := make([]Square, 0, absInt(to.File()-from.File())+1)
	for file := from.File(); ; file += step {
		path = append(path, MustSquare(file, from.Rank()))
		if file == to.File() {
			break
		}
	}

	return path
}

func (l CastlingLayout) KingPath(color Color, kind MoveKind) []Square {
	return rankPath(l.KingStart(color), l.KingEnd(color, kind))
}

func (l CastlingLayout) RookPath(color Color, kind MoveKind) []Square {
	return rankPath(l.RookStart(color, kind), l.RookEnd(color, kind))
}
