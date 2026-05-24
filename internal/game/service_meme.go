package game

import "meme_chess/internal/analyzer/analysis"

func (s *Service) decorateMoveWithMeme(gameID string, session *Session, state State, result MoveResult) MoveResult {
	moveNumber := len(state.Moves)
	if moveNumber == 0 {
		return result
	}

	analysisResult := s.lookupMemeAnalysis(gameID, session, state.GameMode, result.Move, moveNumber)
	classicPosition := currentClassicPositionFromSession(session)
	assignment := selectMoveMeme(
		gameID,
		result.Move,
		moveNumber,
		classifyMoveMemeCategoryWithClassicPosition(state.GameMode, result, analysisResult, classicPosition),
		state.Moves,
	)
	if assignment.ID == "" {
		return result
	}

	session.SetMoveMeme(moveNumber, assignment.ID, assignment.Category)
	result.MemeID = assignment.ID
	result.MemeCategory = assignment.Category
	return result
}

func (s *Service) lookupMemeAnalysis(gameID string, session *Session, gameMode string, move string, moveNumber int) *analysis.Result {
	mode := normalizeGameMode(gameMode)
	if (mode != GameModeClassic && mode != GameModeMeme) || s.moveAnalyzer == nil {
		return nil
	}

	if err := s.syncAnalyzerStack(gameID, session); err != nil {
		return nil
	}

	result, err := s.moveAnalyzer.AnalyzeRecordedMove(gameID, move, moveNumber, 3)
	if err != nil {
		return nil
	}

	return result
}
