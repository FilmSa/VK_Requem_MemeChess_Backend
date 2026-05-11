package game

import (
	"strings"

	"meme_chess/internal/analyzer/position"
	"meme_chess/internal/analyzer/rules"
)

type evolutionRuntime struct {
	state           *position.GameState
	turns           int
	kingRevengeUsed map[position.Color]bool
	rng             randomizer
}

type evolutionAbilities struct {
	pawnCounter  bool
	kingRevenge  bool
	doubleKnight bool
	bishopPierce bool
	rookRampage  bool
}

type atomicOutcome struct {
	move       position.Move
	movedPiece position.Piece
	captured   bool
	effects    []MoveEffect
}

func newEvolutionRuntime(rng randomizer) engineRuntime {
	return &evolutionRuntime{
		state:           position.NewInitial(),
		kingRevengeUsed: make(map[position.Color]bool, 2),
		rng:             rng,
	}
}

func (e *evolutionRuntime) CurrentFEN() string {
	return e.state.FEN()
}

func (e *evolutionRuntime) LegalMoves() []string {
	return generateEvolutionLegalMoves(e)
}

func (e *evolutionRuntime) ApplyMove(raw string) (MoveResult, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return MoveResult{}, ErrInvalidMove
	}
	if e.kingRevengeUsed == nil {
		e.kingRevengeUsed = make(map[position.Color]bool, 2)
	}

	parts := splitMoveSequence(normalized)
	if len(parts) == 0 || len(parts) > 2 {
		return MoveResult{}, ErrInvalidMove
	}

	abilitiesBefore := e.abilitiesForTurn(e.turns)
	work := e.state.Clone()

	captured := false
	effects := make([]MoveEffect, 0, 4)
	if len(parts) == 1 {
		outcome, err := e.applyAtomic(work, parts[0], abilitiesBefore, false)
		if err != nil {
			return MoveResult{}, ErrInvalidMove
		}
		captured = outcome.captured
		effects = append(effects, outcome.effects...)
	} else {
		if !abilitiesBefore.doubleKnight {
			return MoveResult{}, ErrInvalidMove
		}

		first, err := e.applyAtomic(work, parts[0], abilitiesBefore, true)
		if err != nil {
			return MoveResult{}, ErrInvalidMove
		}
		if first.movedPiece.Type != position.Knight {
			return MoveResult{}, ErrInvalidMove
		}

		secondMove, err := position.ParseUCIMove(work, parts[1])
		if err != nil || secondMove.From != first.move.To {
			return MoveResult{}, ErrInvalidMove
		}
		secondPiece := work.PieceAt(secondMove.From)
		if secondPiece.Type != position.Knight || secondPiece.Color != first.movedPiece.Color {
			return MoveResult{}, ErrInvalidMove
		}

		second, err := e.applyAtomic(work, parts[1], abilitiesBefore, false)
		if err != nil {
			return MoveResult{}, ErrInvalidMove
		}
		captured = first.captured || second.captured
		effects = append(effects, first.effects...)
		effects = append(effects, second.effects...)
		effects = append(effects, MoveEffect{
			Type:    EffectTypeKnightDouble,
			Title:   "Knight moved twice",
			Message: "The evolved knight chained two jumps in one turn.",
			Piece:   pieceTypeName(first.movedPiece.Type),
			Color:   colorName(first.movedPiece.Color),
			From:    first.move.From.String(),
			To:      second.move.To.String(),
			Animation: &AnimationHint{
				Name:       "knight-double-jump",
				DurationMs: 420,
				Easing:     "ease-in-out",
			},
		})
	}

	e.state = work
	e.turns++

	abilitiesAfter := e.abilitiesForTurn(e.turns)
	ruleSet := rules.NewEvolutionRuleSet(abilitiesAfter.rookRampage, abilitiesAfter.bishopPierce)
	isCheck := ruleSet.IsCheck(e.state, e.state.SideToMove)
	isCheckmate := isCheck && !e.hasLegalTurn(e.state, abilitiesAfter)

	defendingSide := e.state.SideToMove
	if abilitiesAfter.kingRevenge && isCheckmate && !e.kingRevengeUsed[defendingSide] {
		checkers := evolutionCheckingPieces(e.state, e.state.SideToMove, ruleSet)
		if len(checkers) == 1 {
			checkingPiece := e.state.PieceAt(checkers[0])
			e.removePieceWithCastlingImpact(e.state, checkers[0])
			e.kingRevengeUsed[defendingSide] = true
			effects = append(effects, MoveEffect{
				Type:    EffectTypeKingRevenge,
				Title:   "King revenge",
				Message: "The mating piece was erased because the checkmate was delivered by a single attacker.",
				Piece:   pieceTypeName(position.King),
				Color:   colorName(defendingSide),
				To:      checkers[0].String(),
				Removed: []AffectedPiece{
					affectedPiece(checkers[0], checkingPiece, "enemy", 0, -24),
				},
				Animation: &AnimationHint{
					Name:       "king-revenge-burst",
					DurationMs: 360,
					Easing:     "ease-out",
				},
			})
			isCheck = ruleSet.IsCheck(e.state, e.state.SideToMove)
			isCheckmate = isCheck && !e.hasLegalTurn(e.state, abilitiesAfter)
		}
	}

	return MoveResult{
		FEN:         e.state.FEN(),
		Move:        strings.Join(parts, ","),
		IsCapture:   captured,
		IsCheck:     isCheck,
		IsCheckmate: isCheckmate,
		Effects:     cloneEffects(effects),
	}, nil
}

func (e *evolutionRuntime) abilitiesForTurn(turns int) evolutionAbilities {
	return evolutionAbilities{
		pawnCounter:  turns >= 5,
		kingRevenge:  turns >= 7,
		doubleKnight: turns >= 10,
		bishopPierce: turns >= 15,
		rookRampage:  turns >= 20,
	}
}

func splitMoveSequence(raw string) []string {
	chunks := strings.Split(raw, ",")
	out := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk != "" {
			out = append(out, chunk)
		}
	}
	return out
}

func (e *evolutionRuntime) applyAtomic(gs *position.GameState, raw string, abilities evolutionAbilities, sameTurn bool) (atomicOutcome, error) {
	mv, err := position.ParseUCIMove(gs, raw)
	if err != nil {
		return atomicOutcome{}, err
	}

	piece := gs.PieceAt(mv.From)
	if piece.IsZero() {
		return atomicOutcome{}, ErrInvalidMove
	}

	ruleSet := rules.NewEvolutionRuleSet(abilities.rookRampage, abilities.bishopPierce)
	if err := ruleSet.IsLegalMove(gs, mv); err != nil {
		return atomicOutcome{}, err
	}

	work := gs.Clone()
	outcome, err := e.executeAtomic(work, mv, piece, abilities)
	if err != nil {
		return atomicOutcome{}, err
	}

	if sameTurn {
		restoreIntermediateTurn(work, piece.Color)
	}

	if ruleSet.IsCheck(work, piece.Color) {
		return atomicOutcome{}, ErrInvalidMove
	}

	*gs = *work
	return outcome, nil
}

func (e *evolutionRuntime) executeAtomic(gs *position.GameState, mv position.Move, piece position.Piece, abilities evolutionAbilities) (atomicOutcome, error) {
	if abilities.rookRampage && piece.Type == position.Rook {
		captured, effects, err := e.executeRookRampageMove(gs, mv)
		if err != nil {
			return atomicOutcome{}, err
		}
		return atomicOutcome{move: mv, movedPiece: piece, captured: captured, effects: effects}, nil
	}

	capturedPiece := capturedPieceForMove(gs, mv)
	effects := make([]MoveEffect, 0, 1)
	if abilities.bishopPierce && piece.Type == position.Bishop {
		if skipped := piercedPawnSquares(gs, mv.From, mv.To); len(skipped) > 0 {
			effects = append(effects, MoveEffect{
				Type:    EffectTypeBishopPierce,
				Title:   "Bishop pierced pawns",
				Message: "The evolved bishop attacked through pawns on its diagonal.",
				Piece:   pieceTypeName(piece.Type),
				Color:   colorName(piece.Color),
				From:    mv.From.String(),
				To:      mv.To.String(),
				Animation: &AnimationHint{
					Name:       "bishop-pierce",
					DurationMs: 320,
					Easing:     "ease-out",
				},
			})
		}
	}
	if err := gs.ApplyMove(mv); err != nil {
		return atomicOutcome{}, err
	}

	captured := !capturedPiece.IsZero()
	if abilities.pawnCounter && piece.Type == position.Pawn && capturedPiece.Type == position.Pawn {
		hit, err := e.rng.Intn(2)
		if err != nil {
			return atomicOutcome{}, err
		}
		if hit == 0 {
			counteredPiece := gs.PieceAt(mv.To)
			captured = e.removePieceWithCastlingImpact(gs, mv.To) || captured
			effects = append(effects, MoveEffect{
				Type:    EffectTypePawnCounter,
				Title:   "Pawn counter",
				Message: "The captured pawn struck back and removed the attacking pawn.",
				Piece:   pieceTypeName(piece.Type),
				Color:   colorName(piece.Color),
				To:      mv.To.String(),
				Removed: []AffectedPiece{
					affectedPiece(mv.To, counteredPiece, "self", 0, -18),
				},
				Animation: &AnimationHint{
					Name:       "pawn-counter",
					DurationMs: 260,
					Easing:     "ease-out",
				},
			})
		}
	}

	return atomicOutcome{move: mv, movedPiece: piece, captured: captured, effects: effects}, nil
}

func (e *evolutionRuntime) executeRookRampageMove(gs *position.GameState, mv position.Move) (bool, []MoveEffect, error) {
	destinationPiece := gs.PieceAt(mv.To)
	captured := !destinationPiece.IsZero()
	if err := gs.ApplyMove(mv); err != nil {
		return false, nil, err
	}

	removed := make([]AffectedPiece, 0, 8)
	if !destinationPiece.IsZero() {
		kx, ky := rookKnockback(0, mv.From, mv.To)
		removed = append(removed, affectedPiece(mv.To, destinationPiece, "enemy", kx, ky))
	}

	for _, sq := range lineSquaresExclusive(mv.From, mv.To) {
		piece := gs.PieceAt(sq)
		if piece.IsZero() {
			continue
		}
		if piece.Type == position.King {
			return false, nil, ErrInvalidMove
		}
		if piece.Color == gs.PieceAt(mv.To).Color {
			continue
		}
		if e.removePieceWithCastlingImpact(gs, sq) {
			captured = true
			kx, ky := rookKnockback(len(removed), mv.From, mv.To)
			removed = append(removed, affectedPiece(sq, piece, "enemy", kx, ky))
		}
	}

	if captured {
		gs.HalfmoveClock = 0
	}

	return captured, []MoveEffect{
		{
			Type:    EffectTypeRookRampage,
			Title:   "Rook rampage",
			Message: "The evolved rook plowed through every enemy piece on its line and can check through blockers.",
			Piece:   pieceTypeName(position.Rook),
			Color:   colorName(gs.PieceAt(mv.To).Color),
			From:    mv.From.String(),
			To:      mv.To.String(),
			Removed: removed,
			Animation: &AnimationHint{
				Name:       "rook-rampage",
				DurationMs: 560,
				Easing:     "ease-in-out",
			},
		},
	}, nil
}

func (e *evolutionRuntime) removePieceWithCastlingImpact(gs *position.GameState, sq position.Square) bool {
	piece := gs.PieceAt(sq)
	if piece.IsZero() || piece.Type == position.King {
		return false
	}

	layout := gs.CastlingLayoutValue()
	if piece.Type == position.Rook {
		switch {
		case piece.Color == position.White && sq == layout.RookStart(position.White, position.MoveCastleKingSide):
			gs.CastlingRights.WhiteKingSide = false
		case piece.Color == position.White && sq == layout.RookStart(position.White, position.MoveCastleQueenSide):
			gs.CastlingRights.WhiteQueenSide = false
		case piece.Color == position.Black && sq == layout.RookStart(position.Black, position.MoveCastleKingSide):
			gs.CastlingRights.BlackKingSide = false
		case piece.Color == position.Black && sq == layout.RookStart(position.Black, position.MoveCastleQueenSide):
			gs.CastlingRights.BlackQueenSide = false
		}
	}

	gs.SetPiece(sq, position.Piece{})
	return true
}

func restoreIntermediateTurn(gs *position.GameState, mover position.Color) {
	if gs == nil {
		return
	}
	gs.SideToMove = mover
	if mover == position.Black {
		gs.FullmoveNumber--
	}
}

func lineSquaresExclusive(from, to position.Square) []position.Square {
	df := sign(to.File() - from.File())
	dr := sign(to.Rank() - from.Rank())

	out := make([]position.Square, 0, 7)
	for file, rank := from.File()+df, from.Rank()+dr; file != to.File() || rank != to.Rank(); file, rank = file+df, rank+dr {
		out = append(out, position.MustSquare(file, rank))
	}
	return out
}

func piercedPawnSquares(fromState *position.GameState, from, to position.Square) []position.Square {
	df := sign(to.File() - from.File())
	dr := sign(to.Rank() - from.Rank())

	out := make([]position.Square, 0, 7)
	for file, rank := from.File()+df, from.Rank()+dr; file != to.File() || rank != to.Rank(); file, rank = file+df, rank+dr {
		sq := position.MustSquare(file, rank)
		if fromState.PieceAt(sq).Type == position.Pawn {
			out = append(out, sq)
		}
	}
	return out
}

func sign(v int) int {
	if v < 0 {
		return -1
	}
	if v > 0 {
		return 1
	}
	return 0
}

func evolutionCheckingPieces(gs *position.GameState, color position.Color, rs *rules.EvolutionRuleSet) []position.Square {
	kingSq, ok := findKingSquare(gs, color)
	if !ok {
		return nil
	}

	enemy := color.Opponent()
	checkers := make([]position.Square, 0, 2)
	for i := 0; i < 64; i++ {
		sq := position.Square(i)
		piece := gs.PieceAt(sq)
		if piece.IsZero() || piece.Color != enemy {
			continue
		}
		if rs.IsCheck(gs, color) && rsAttacksSquare(rs, gs, sq, kingSq, piece) {
			checkers = append(checkers, sq)
		}
	}
	return checkers
}

func rsAttacksSquare(rs *rules.EvolutionRuleSet, gs *position.GameState, from, to position.Square, piece position.Piece) bool {
	if piece.Type == position.Rook && rs.RookRampage {
		df := to.File() - from.File()
		dr := to.Rank() - from.Rank()
		if (df != 0 && dr != 0) || (df == 0 && dr == 0) {
			return false
		}
		for _, sq := range lineSquaresExclusive(from, to) {
			if gs.PieceAt(sq).Type == position.King {
				return false
			}
		}
		return true
	}
	return rules.AttacksSquare(gs, from, to, piece)
}

func findKingSquare(gs *position.GameState, color position.Color) (position.Square, bool) {
	for i := 0; i < 64; i++ {
		sq := position.Square(i)
		piece := gs.PieceAt(sq)
		if !piece.IsZero() && piece.Type == position.King && piece.Color == color {
			return sq, true
		}
	}
	return position.NoSquare, false
}

func (e *evolutionRuntime) hasLegalTurn(gs *position.GameState, abilities evolutionAbilities) bool {
	side := gs.SideToMove

	for _, raw := range candidateMoves(gs, side, position.NoSquare) {
		tmp := gs.Clone()
		if _, err := e.applyAtomic(tmp, raw, abilities, false); err == nil {
			return true
		}
	}

	if !abilities.doubleKnight {
		return false
	}

	for i := 0; i < 64; i++ {
		from := position.Square(i)
		piece := gs.PieceAt(from)
		if piece.IsZero() || piece.Color != side || piece.Type != position.Knight {
			continue
		}

		for _, firstRaw := range candidateMoves(gs, side, from) {
			tmpFirst := gs.Clone()
			first, err := e.applyAtomic(tmpFirst, firstRaw, abilities, true)
			if err != nil {
				continue
			}

			current := tmpFirst.PieceAt(first.move.To)
			if current.Type != position.Knight || current.Color != side {
				continue
			}

			for _, secondRaw := range candidateMoves(tmpFirst, side, first.move.To) {
				tmpSecond := tmpFirst.Clone()
				if _, err := e.applyAtomic(tmpSecond, secondRaw, abilities, false); err == nil {
					return true
				}
			}
		}
	}

	return false
}

func candidateMoves(gs *position.GameState, side position.Color, onlyFrom position.Square) []string {
	out := make([]string, 0, 256)
	for i := 0; i < 64; i++ {
		from := position.Square(i)
		if onlyFrom != position.NoSquare && from != onlyFrom {
			continue
		}

		piece := gs.PieceAt(from)
		if piece.IsZero() || piece.Color != side {
			continue
		}

		for j := 0; j < 64; j++ {
			to := position.Square(j)
			if to == from {
				continue
			}

			if piece.Type == position.Pawn && (to.Rank() == 0 || to.Rank() == 7) {
				out = append(out,
					uciForMove(from, to, "q"),
					uciForMove(from, to, "r"),
					uciForMove(from, to, "b"),
					uciForMove(from, to, "n"),
				)
				continue
			}

			out = append(out, uciForMove(from, to, ""))
		}
	}
	return out
}

func uciForMove(from, to position.Square, suffix string) string {
	return from.String() + to.String() + suffix
}
