package game

import "strings"

const (
	GameModeClassic   = "classic"
	GameModeMeme      = "meme"
	GameModeFischer   = "fischer"
	GameModeEvolution = "evolution"
)

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
