package game

import (
	"testing"

	"meme_chess/internal/analyzer/analysis"
	"meme_chess/internal/analyzer/pattern"
	"meme_chess/internal/analyzer/position"
)

func TestSelectMoveMemeIsDeterministic(t *testing.T) {
	first := selectMoveMeme("game-1", "e2e4", 1, memeCategoryDevelopment, nil)
	second := selectMoveMeme("game-1", "e2e4", 1, memeCategoryDevelopment, nil)

	if first.ID == "" {
		t.Fatal("expected deterministic meme id to be assigned")
	}
	if first != second {
		t.Fatalf("expected deterministic assignment, got %+v and %+v", first, second)
	}
}

func TestSelectMoveMemeAvoidsImmediateRepeatWhenAlternativesExist(t *testing.T) {
	previous := []Move{
		{
			Number:       1,
			Move:         "e2e4",
			MemeID:       memeCatalogByCategory[memeCategoryDevelopment][0].ID,
			MemeCategory: memeCategoryDevelopment,
		},
		{
			Number: 2,
			Move:   "e7e5",
		},
	}

	assignment := selectMoveMeme(
		"game-1",
		"g1f3",
		2,
		memeCategoryDevelopment,
		previous,
	)

	if assignment.ID == "" {
		t.Fatal("expected meme id to be assigned")
	}
	if assignment.ID == previous[0].MemeID {
		t.Fatalf("expected a different meme than the previous move, got %q", assignment.ID)
	}
}

func TestLeastUsedMemeDefinitionsPrefersRarerMemes(t *testing.T) {
	definitions := memeCatalogByCategory[memeCategoryDevelopment][:3]
	moves := []Move{
		{MemeID: definitions[0].ID},
		{MemeID: definitions[0].ID},
		{MemeID: definitions[1].ID},
	}

	filtered := leastUsedMemeDefinitions(definitions, moves)

	if len(filtered) != 1 {
		t.Fatalf("expected exactly one least-used candidate, got %d", len(filtered))
	}
	if filtered[0].ID != definitions[2].ID {
		t.Fatalf("expected least-used meme %q, got %q", definitions[2].ID, filtered[0].ID)
	}
}

func TestClassifyClassicMoveMemeCategoryPrefersCheckOverCapture(t *testing.T) {
	category := classifyClassicMoveMemeCategory(
		MoveResult{IsCapture: true, IsCheck: true},
		&analysis.Result{
			Tags: []pattern.Tag{pattern.TagWinMaterial, pattern.TagCheck},
		},
	)

	if category != memeCategoryCheck {
		t.Fatalf("expected %q, got %q", memeCategoryCheck, category)
	}
}

func TestClassifyClassicMoveMemeCategoryPrefersRuntimeSacrificeOverCheck(t *testing.T) {
	state, err := position.BuildGameStateFromUCIMoves([]string{
		"e2e4",
		"e7e5",
		"f1c4",
		"b8c6",
		"c4f7",
	})
	if err != nil {
		t.Fatalf("build game state: %v", err)
	}

	category := classifyClassicMoveMemeCategoryWithPosition(
		MoveResult{IsCapture: true, IsCheck: true},
		nil,
		state,
	)

	if category != memeCategorySacrifice {
		t.Fatalf("expected %q, got %q", memeCategorySacrifice, category)
	}
}

func TestClassifyClassicMoveMemeCategoryDoesNotTreatDefendedCheckAsSacrifice(t *testing.T) {
	state, err := position.BuildGameStateFromUCIMoves([]string{
		"e2e4",
		"e7e5",
		"d1h5",
		"b8c6",
		"f1c4",
		"g8f6",
		"h5f7",
	})
	if err != nil {
		t.Fatalf("build game state: %v", err)
	}

	category := classifyClassicMoveMemeCategoryWithPosition(
		MoveResult{IsCapture: true, IsCheck: true},
		nil,
		state,
	)

	if category != memeCategoryCheck {
		t.Fatalf("expected %q, got %q", memeCategoryCheck, category)
	}
}

func TestClassifyClassicMoveMemeCategoryDoesNotTreatDefendedMaterialGainAsSacrifice(t *testing.T) {
	state, err := position.BuildGameStateFromUCIMoves([]string{
		"e2e4",
		"e7e5",
		"d2d4",
		"e5d4",
		"g1f3",
		"b8c6",
		"f3d4",
	})
	if err != nil {
		t.Fatalf("build game state: %v", err)
	}

	category := classifyClassicMoveMemeCategoryWithPosition(
		MoveResult{IsCapture: true},
		nil,
		state,
	)

	if category != memeCategoryImportantCapture {
		t.Fatalf("expected %q, got %q", memeCategoryImportantCapture, category)
	}
}

func TestNeedsStoredMoveMemeBackfill(t *testing.T) {
	if !needsStoredMoveMemeBackfill(memeAssignment{}) {
		t.Fatal("expected empty assignment to require backfill")
	}

	if needsStoredMoveMemeBackfill(memeAssignment{
		ID:       memeCatalogByCategory[memeCategoryDevelopment][0].ID,
		Category: memeCategoryDevelopment,
	}) {
		t.Fatal("expected known meme assignment to be preserved")
	}

	if !needsStoredMoveMemeBackfill(memeAssignment{
		ID:       memeCatalogByCategory[memeCategoryDevelopment][0].ID,
		Category: memeCategoryCheck,
	}) {
		t.Fatal("expected mismatched meme id/category pair to require backfill")
	}
}
