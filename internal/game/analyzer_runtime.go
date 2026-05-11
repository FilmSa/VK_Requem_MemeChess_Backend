package game

import (
	"fmt"
	"slices"
	"strings"

	analyzergame "meme_chess/internal/analyzer/game"
	"meme_chess/internal/analyzer/movegen"
	"meme_chess/internal/analyzer/position"
	"meme_chess/internal/analyzer/rules"
)

type analyzerRuntime struct {
	state *position.GameState
	rules rules.RuleSet
}

func newEngineRuntime(mode string, rng randomizer) (engineRuntime, error) {
	switch normalizeGameMode(mode) {
	case GameModeClassic, GameModeMeme:
		return &analyzerRuntime{
			state: position.NewInitial(),
			rules: rules.NewClassicalRuleSet(),
		}, nil
	case GameModeFischer:
		return newFischerRuntime(rng)
	case GameModeEvolution:
		return newEvolutionRuntime(rng), nil
	default:
		return nil, fmt.Errorf("unsupported game mode %q", mode)
	}
}

func (r *analyzerRuntime) CurrentFEN() string {
	return r.state.FEN()
}

func (r *analyzerRuntime) ApplyMove(raw string) (MoveResult, error) {
	return applySingleMove(r.state, r.rules, strings.TrimSpace(strings.ToLower(raw)))
}

func (r *analyzerRuntime) LegalMoves() []string {
	moves := movegen.NewGenerator(r.rules).GenerateLegalMoves(r.state)
	if len(moves) == 0 {
		return nil
	}

	result := make([]string, 0, len(moves))
	for _, move := range moves {
		result = append(result, encodeUCIMove(move))
	}

	return result
}

func applySingleMove(gs *position.GameState, rs rules.RuleSet, raw string) (MoveResult, error) {
	if raw == "" {
		return MoveResult{}, ErrInvalidMove
	}

	mv, err := position.ParseUCIMove(gs, raw)
	if err != nil {
		return MoveResult{}, ErrInvalidMove
	}

	if err := rs.IsLegalMove(gs, mv); err != nil {
		return MoveResult{}, ErrInvalidMove
	}

	captured := capturedPieceForMove(gs, mv)
	if err := gs.ApplyMove(mv); err != nil {
		return MoveResult{}, ErrInvalidMove
	}

	return MoveResult{
		FEN:         gs.FEN(),
		Move:        raw,
		IsCapture:   !captured.IsZero(),
		IsCheck:     rs.IsCheck(gs, gs.SideToMove),
		IsCheckmate: analyzergame.IsCheckmate(gs, rs),
		Effects:     nil,
	}, nil
}

func capturedPieceForMove(gs *position.GameState, mv position.Move) position.Piece {
	if gs == nil {
		return position.Piece{}
	}

	if mv.Kind == position.MoveEnPassant {
		moved := gs.PieceAt(mv.From)
		if moved.Color == position.White {
			return gs.PieceAt(position.MustSquare(mv.To.File(), mv.To.Rank()-1))
		}
		return gs.PieceAt(position.MustSquare(mv.To.File(), mv.To.Rank()+1))
	}

	return gs.PieceAt(mv.To)
}

func newFischerRuntime(rng randomizer) (engineRuntime, error) {
	files := []int{0, 1, 2, 3, 4, 5, 6, 7}
	backRank := make([]position.PieceType, 8)

	darkIndex, err := chooseIndex(rng, []int{0, 2, 4, 6})
	if err != nil {
		return nil, err
	}
	backRank[darkIndex] = position.Bishop
	files = removeFile(files, darkIndex)

	lightIndex, err := chooseIndex(rng, []int{1, 3, 5, 7})
	if err != nil {
		return nil, err
	}
	backRank[lightIndex] = position.Bishop
	files = removeFile(files, lightIndex)

	queenPos, err := chooseRemaining(rng, files)
	if err != nil {
		return nil, err
	}
	backRank[queenPos] = position.Queen
	files = removeFile(files, queenPos)

	knightOne, err := chooseRemaining(rng, files)
	if err != nil {
		return nil, err
	}
	backRank[knightOne] = position.Knight
	files = removeFile(files, knightOne)

	knightTwo, err := chooseRemaining(rng, files)
	if err != nil {
		return nil, err
	}
	backRank[knightTwo] = position.Knight
	files = removeFile(files, knightTwo)

	slices.Sort(files)
	backRank[files[0]] = position.Rook
	backRank[files[1]] = position.King
	backRank[files[2]] = position.Rook

	layout := &position.CastlingLayout{
		White: position.CastlingSideLayout{
			KingStart:          position.MustSquare(files[1], 0),
			QueenSideRookStart: position.MustSquare(files[0], 0),
			KingSideRookStart:  position.MustSquare(files[2], 0),
		},
		Black: position.CastlingSideLayout{
			KingStart:          position.MustSquare(files[1], 7),
			QueenSideRookStart: position.MustSquare(files[0], 7),
			KingSideRookStart:  position.MustSquare(files[2], 7),
		},
	}

	state := &position.GameState{
		SideToMove: position.White,
		CastlingRights: position.CastlingRights{
			WhiteKingSide:  true,
			WhiteQueenSide: true,
			BlackKingSide:  true,
			BlackQueenSide: true,
		},
		CastlingLayout: layout,
		EnPassant:      position.NoSquare,
		HalfmoveClock:  0,
		FullmoveNumber: 1,
	}

	for file, pieceType := range backRank {
		state.SetPiece(position.MustSquare(file, 0), position.Piece{Type: pieceType, Color: position.White})
		state.SetPiece(position.MustSquare(file, 1), position.Piece{Type: position.Pawn, Color: position.White})
		state.SetPiece(position.MustSquare(file, 6), position.Piece{Type: position.Pawn, Color: position.Black})
		state.SetPiece(position.MustSquare(file, 7), position.Piece{Type: pieceType, Color: position.Black})
	}

	return &analyzerRuntime{
		state: state,
		rules: rules.NewClassicalRuleSet(),
	}, nil
}

func chooseIndex(rng randomizer, candidates []int) (int, error) {
	pick, err := rng.Intn(len(candidates))
	if err != nil {
		return 0, err
	}
	return candidates[pick], nil
}

func chooseRemaining(rng randomizer, remaining []int) (int, error) {
	pick, err := rng.Intn(len(remaining))
	if err != nil {
		return 0, err
	}
	return remaining[pick], nil
}

func removeFile(files []int, target int) []int {
	out := make([]int, 0, len(files)-1)
	for _, file := range files {
		if file != target {
			out = append(out, file)
		}
	}
	return out
}
