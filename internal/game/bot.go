package game

import (
	"errors"
	"fmt"
	"strings"

	"meme_chess/internal/analyzer/moveeval"
	"meme_chess/internal/analyzer/position"
	"meme_chess/internal/analyzer/rules"
	"meme_chess/internal/analyzer/search"
)

const (
	botUserID      = "00000000-0000-0000-0000-00000000b007"
	botUsername    = "MemeBot"
	botEasy        = "easy"
	botMedium      = "medium"
	botHard        = "hard"
	botEasyDepth   = 3
	botMediumDepth = 6
	botHardDepth   = 9
	botMateScore   = 1000000
	botNegativeInf = -10000000
	botPositiveInf = 10000000
)

var errNoLegalBotMove = errors.New("bot has no legal move")

func normalizeBotDifficulty(value string) (string, int, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", botEasy:
		return botEasy, botEasyDepth, true
	case botMedium:
		return botMedium, botMediumDepth, true
	case botHard:
		return botHard, botHardDepth, true
	default:
		return "", 0, false
	}
}

func botDisplayName() string {
	return botUsername
}

func isBotUserID(userID string) bool {
	return strings.TrimSpace(userID) == botUserID
}

func chooseBotMove(engine Engine, difficulty string) (string, error) {
	_, depth, ok := normalizeBotDifficulty(difficulty)
	if !ok {
		return "", ErrInvalidDifficulty
	}

	chessEngine, ok := engine.(*ChessEngine)
	if !ok {
		return "", fmt.Errorf("unsupported bot engine")
	}

	switch runtime := chessEngine.runtime.(type) {
	case *analyzerRuntime:
		return chooseAnalyzerMove(runtime, depth)
	case *evolutionRuntime:
		return chooseEvolutionMove(runtime, depth)
	default:
		return "", fmt.Errorf("unsupported runtime")
	}
}

func chooseAnalyzerMove(runtime *analyzerRuntime, depth int) (string, error) {
	root := search.NewEngine(runtime.rules).AnalyzePosition(runtime.state.Clone(), depth)
	if len(root.RootMoves) == 0 {
		return "", errNoLegalBotMove
	}

	return encodeUCIMove(root.BestMove), nil
}

func encodeUCIMove(mv position.Move) string {
	raw := mv.From.String() + mv.To.String()
	if mv.Promotion != position.NoPieceType {
		raw += botPromotionSuffix(mv.Promotion)
	}
	return raw
}

func botPromotionSuffix(pieceType position.PieceType) string {
	switch pieceType {
	case position.Queen:
		return "q"
	case position.Rook:
		return "r"
	case position.Bishop:
		return "b"
	case position.Knight:
		return "n"
	default:
		return ""
	}
}

type deterministicRandomizer struct {
	value int
}

func (r deterministicRandomizer) Intn(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("invalid bound %d", n)
	}
	return r.value % n, nil
}

func (e *evolutionRuntime) cloneForSearch() *evolutionRuntime {
	kingRevengeUsed := make(map[position.Color]bool, len(e.kingRevengeUsed))
	for color, used := range e.kingRevengeUsed {
		kingRevengeUsed[color] = used
	}

	return &evolutionRuntime{
		state:           e.state.Clone(),
		turns:           e.turns,
		kingRevengeUsed: kingRevengeUsed,
		rng:             deterministicRandomizer{},
	}
}

func chooseEvolutionMove(runtime *evolutionRuntime, depth int) (string, error) {
	moves := generateEvolutionLegalMoves(runtime)
	if len(moves) == 0 {
		return "", errNoLegalBotMove
	}

	bestMove := moves[0]
	bestScore := botNegativeInf
	alpha := botNegativeInf
	beta := botPositiveInf

	for _, raw := range moves {
		next := runtime.cloneForSearch()
		result, err := next.ApplyMove(raw)
		if err != nil {
			continue
		}

		score := botMateScore - 1
		if !result.IsCheckmate {
			score = -evolutionNegamax(next, depth-1, 1, -beta, -alpha)
		}

		if score > bestScore {
			bestScore = score
			bestMove = raw
		}
		if score > alpha {
			alpha = score
		}
	}

	return bestMove, nil
}

func evolutionNegamax(runtime *evolutionRuntime, depth int, ply int, alpha int, beta int) int {
	abilities := runtime.abilitiesForTurn(runtime.turns)
	moves := generateEvolutionLegalMoves(runtime)
	if len(moves) == 0 {
		ruleSet := rules.NewEvolutionRuleSet(abilities.rookRampage, abilities.bishopPierce)
		if ruleSet.IsCheck(runtime.state, runtime.state.SideToMove) {
			return -botMateScore + ply
		}
		return 0
	}

	if depth <= 0 {
		return moveeval.Evaluate(runtime.state)
	}

	bestScore := botNegativeInf
	for _, raw := range moves {
		next := runtime.cloneForSearch()
		result, err := next.ApplyMove(raw)
		if err != nil {
			continue
		}

		score := botMateScore - ply
		if !result.IsCheckmate {
			score = -evolutionNegamax(next, depth-1, ply+1, -beta, -alpha)
		}

		if score > bestScore {
			bestScore = score
		}
		if score > alpha {
			alpha = score
		}
		if alpha >= beta {
			break
		}
	}

	return bestScore
}

func generateEvolutionLegalMoves(runtime *evolutionRuntime) []string {
	abilities := runtime.abilitiesForTurn(runtime.turns)
	side := runtime.state.SideToMove
	seen := make(map[string]struct{}, 128)
	legal := make([]string, 0, 128)

	appendMove := func(raw string) {
		if raw == "" {
			return
		}
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		legal = append(legal, raw)
	}

	for _, raw := range candidateMoves(runtime.state, side, position.NoSquare) {
		next := runtime.cloneForSearch()
		if _, err := next.applyAtomic(next.state, raw, abilities, false); err == nil {
			appendMove(raw)
		}
	}

	if !abilities.doubleKnight {
		return legal
	}

	for i := 0; i < 64; i++ {
		from := position.Square(i)
		piece := runtime.state.PieceAt(from)
		if piece.IsZero() || piece.Color != side || piece.Type != position.Knight {
			continue
		}

		for _, firstRaw := range candidateMoves(runtime.state, side, from) {
			firstRuntime := runtime.cloneForSearch()
			first, err := firstRuntime.applyAtomic(firstRuntime.state, firstRaw, abilities, true)
			if err != nil {
				continue
			}

			current := firstRuntime.state.PieceAt(first.move.To)
			if current.IsZero() || current.Color != side || current.Type != position.Knight {
				continue
			}

			for _, secondRaw := range candidateMoves(firstRuntime.state, side, first.move.To) {
				secondRuntime := firstRuntime.cloneForSearch()
				if _, err := secondRuntime.applyAtomic(secondRuntime.state, secondRaw, abilities, false); err == nil {
					appendMove(firstRaw + "," + secondRaw)
				}
			}
		}
	}

	return legal
}
