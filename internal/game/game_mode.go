package game

import "strings"

const (
	GameModeClassic   = "classic"
	GameModeMeme      = "meme"
	GameModeFischer   = "fischer"
	GameModeEvolution = "evolution"
)

var (
	QuickGameMemeCategoryModes = []string{GameModeClassic, GameModeMeme}
	QuickGameVariantCategoryModes = []string{
		GameModeClassic,
		GameModeFischer,
		GameModeEvolution,
	}
)

func QuickGameModeCategories() [][]string {
	return [][]string{QuickGameMemeCategoryModes, QuickGameVariantCategoryModes}
}

func isQuickGameMode(mode string) bool {
	switch normalizeGameMode(mode) {
	case GameModeClassic, GameModeMeme, GameModeFischer, GameModeEvolution:
		return true
	default:
		return false
	}
}

func normalizeGameMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", GameModeClassic:
		return GameModeClassic
	case GameModeMeme:
		return GameModeMeme
	case GameModeFischer:
		return GameModeFischer
	case GameModeEvolution:
		return GameModeEvolution
	default:
		return ""
	}
}
