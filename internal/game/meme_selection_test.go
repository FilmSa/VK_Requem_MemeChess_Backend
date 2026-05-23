package game

import (
	"testing"

	"meme_chess/internal/analyzer/analysis"
	"meme_chess/internal/analyzer/pattern"
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
