package game

import (
	"strings"

	"meme_chess/internal/analyzer/movegen"
	"meme_chess/internal/analyzer/position"
	"meme_chess/internal/analyzer/rules"
)

var classicMemeRuleSet = rules.NewClassicalRuleSet()

func currentClassicPositionFromSession(session *Session) *position.GameState {
	if session == nil {
		return nil
	}

	session.mu.RLock()
	defer session.mu.RUnlock()

	chessEngine, ok := session.engine.(*ChessEngine)
	if !ok || chessEngine == nil {
		return nil
	}

	runtime, ok := chessEngine.runtime.(*analyzerRuntime)
	if !ok || runtime == nil || runtime.state == nil {
		return nil
	}

	return runtime.state.Clone()
}

func advanceClassicReplayPosition(gs *position.GameState, raw string) *position.GameState {
	if gs == nil {
		return nil
	}

	move, err := position.ParseUCIMove(gs, strings.TrimSpace(strings.ToLower(raw)))
	if err != nil {
		return nil
	}

	if err := classicMemeRuleSet.IsLegalMove(gs, move); err != nil {
		return nil
	}

	if err := gs.ApplyMove(move); err != nil {
		return nil
	}

	return gs
}

func detectsClassicSacrifice(gs *position.GameState) bool {
	if gs == nil || len(gs.History) == 0 {
		return false
	}

	last := gs.History[len(gs.History)-1]
	if last.IsNull {
		return false
	}

	movedPiece := gs.PieceAt(last.Move.To)
	if movedPiece.IsZero() || !isClassicImportantPieceType(movedPiece.Type) {
		return false
	}

	capturedValue := classicPieceValue(last.CapturedPiece.Type)
	movedPieceValue := classicPieceValue(movedPiece.Type)
	targetSquare := last.Move.To

	for _, enemyCapture := range legalClassicCapturesToSquare(gs, targetSquare) {
		afterEnemyCapture := gs.Clone()
		if err := afterEnemyCapture.ApplyMove(enemyCapture); err != nil {
			continue
		}

		recaptureGain := 0
		if hasLegalClassicCaptureToSquare(afterEnemyCapture, targetSquare) {
			recaptureGain = classicPieceValue(afterEnemyCapture.PieceAt(targetSquare).Type)
		}

		netMaterial := capturedValue - movedPieceValue + recaptureGain
		if netMaterial < 0 {
			return true
		}
	}

	return false
}

func isClassicImportantPieceType(pt position.PieceType) bool {
	switch pt {
	case position.Knight, position.Bishop, position.Rook, position.Queen:
		return true
	default:
		return false
	}
}

func legalClassicCapturesToSquare(gs *position.GameState, target position.Square) []position.Move {
	if gs == nil {
		return nil
	}

	moves := movegen.NewGenerator(classicMemeRuleSet).GenerateLegalMoves(gs)
	if len(moves) == 0 {
		return nil
	}

	captures := make([]position.Move, 0, 4)
	for _, move := range moves {
		if move.To != target {
			continue
		}
		if capturedPieceForMove(gs, move).IsZero() {
			continue
		}
		captures = append(captures, move)
	}

	return captures
}

func hasLegalClassicCaptureToSquare(gs *position.GameState, target position.Square) bool {
	return len(legalClassicCapturesToSquare(gs, target)) > 0
}

func classicPieceValue(pt position.PieceType) int {
	switch pt {
	case position.Pawn:
		return 100
	case position.Knight, position.Bishop:
		return 300
	case position.Rook:
		return 500
	case position.Queen:
		return 900
	case position.King:
		return 100
	default:
		return 0
	}
}
