package game

import (
	"context"
	"fmt"
	"strings"

	"meme_chess/internal/analyzer/analysis"
	"meme_chess/internal/analyzer/position"
)

const backfillMemeAnalysisDepth = 3

func (s *Service) BackfillStoredMoveMemes(ctx context.Context) (int, error) {
	if s.repository == nil {
		return 0, nil
	}

	rows, err := s.repository.ListMovesForMemeBackfill(ctx)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	updated := 0
	currentGameID := ""
	backfillGameID := ""
	stack := make([]string, 0, 128)
	effectiveMoves := make([]Move, 0, 128)
	var classicPosition *position.GameState

	for _, row := range rows {
		if row.GameID != currentGameID {
			s.forgetBackfillAnalyzerGame(backfillGameID)

			currentGameID = row.GameID
			backfillGameID = fmt.Sprintf("meme-backfill:%s", row.GameID)
			stack = stack[:0]
			effectiveMoves = effectiveMoves[:0]
			classicPosition = nil
		}

		stack = append(stack, row.Move)
		if normalizeGameMode(row.GameMode) == GameModeClassic || normalizeGameMode(row.GameMode) == GameModeMeme {
			if classicPosition == nil {
				classicPosition = position.NewInitial()
			}
			classicPosition = advanceClassicReplayPosition(classicPosition, row.Move)
		}
		effectiveAssignment := memeAssignment{
			ID:       strings.TrimSpace(row.MemeID),
			Category: strings.TrimSpace(row.MemeCategory),
		}

		hasRuntimeSacrificeSignal := classicPosition != nil && detectsClassicSacrifice(classicPosition)
		shouldReclassifySacrifice := classicPosition != nil &&
			(effectiveAssignment.Category == memeCategorySacrifice) != hasRuntimeSacrificeSignal

		if needsStoredMoveMemeBackfill(effectiveAssignment) || shouldReclassifySacrifice {
			analysisResult := s.lookupBackfillMoveAnalysis(
				backfillGameID,
				stack,
				row.Move,
				row.MoveNumber,
			)
			result := MoveResult{
				Move:        row.Move,
				FEN:         row.FEN,
				IsCapture:   row.IsCapture,
				IsCheck:     row.IsCheck,
				IsCheckmate: row.IsCheckmate,
			}
			category := classifyMoveMemeCategoryWithClassicPosition(
				row.GameMode,
				result,
				analysisResult,
				classicPosition,
			)
			historyWithCurrent := append(
				append([]Move(nil), effectiveMoves...),
				Move{
					Number:      row.MoveNumber,
					Move:        row.Move,
					FEN:         row.FEN,
					IsCapture:   row.IsCapture,
					IsCheck:     row.IsCheck,
					IsCheckmate: row.IsCheckmate,
				},
			)
			effectiveAssignment = selectMoveMeme(
				row.GameID,
				row.Move,
				row.MoveNumber,
				category,
				historyWithCurrent,
			)
			if effectiveAssignment.ID != "" {
				if err := s.repository.UpdateMoveMeme(
					ctx,
					row.GameID,
					row.MoveNumber,
					effectiveAssignment.ID,
					effectiveAssignment.Category,
				); err != nil {
					return updated, err
				}
				updated++
			}
		}

		effectiveMoves = append(effectiveMoves, Move{
			Number:       row.MoveNumber,
			Move:         row.Move,
			FEN:          row.FEN,
			IsCapture:    row.IsCapture,
			IsCheck:      row.IsCheck,
			IsCheckmate:  row.IsCheckmate,
			MemeID:       effectiveAssignment.ID,
			MemeCategory: effectiveAssignment.Category,
		})
	}

	s.forgetBackfillAnalyzerGame(backfillGameID)
	return updated, nil
}

func needsStoredMoveMemeBackfill(assignment memeAssignment) bool {
	return !isKnownMemeID(assignment.ID) ||
		!isKnownMemeCategory(assignment.Category) ||
		!isKnownMemeAssignment(assignment.ID, assignment.Category)
}

func (s *Service) lookupBackfillMoveAnalysis(
	gameID string,
	moves []string,
	move string,
	moveNumber int,
) *analysis.Result {
	if s.moveAnalyzer == nil || strings.TrimSpace(gameID) == "" || len(moves) == 0 {
		return nil
	}

	type syncer interface {
		SyncGame(gameID string, moves []string) error
	}

	moveAnalyzer, ok := s.moveAnalyzer.(syncer)
	if !ok {
		return nil
	}

	if err := moveAnalyzer.SyncGame(gameID, moves); err != nil {
		return nil
	}

	result, err := s.moveAnalyzer.AnalyzeRecordedMove(
		gameID,
		move,
		moveNumber,
		backfillMemeAnalysisDepth,
	)
	if err != nil {
		return nil
	}

	return result
}

func (s *Service) forgetBackfillAnalyzerGame(gameID string) {
	if s.moveAnalyzer == nil || strings.TrimSpace(gameID) == "" {
		return
	}

	s.moveAnalyzer.ForgetGame(gameID)
}
