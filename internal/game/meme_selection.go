package game

import (
	"hash/fnv"
	"strconv"
	"strings"

	"meme_chess/internal/analyzer/analysis"
	"meme_chess/internal/analyzer/pattern"
	"meme_chess/internal/analyzer/position"
)

const (
	memeCategoryForkPin          = "FORKPIN"
	memeCategoryImportantCapture = "VZYATIEVAZHNOIFIGYRI"
	memeCategoryCheck            = "SHAH"
	memeCategorySacrifice        = "ZHERTVA"
	memeCategoryDevelopment      = "RAZVITIEFIGURI"

	memeRecentRepeatWindow = 6
)

type memeAssignment struct {
	ID       string
	Category string
}

type memeDefinition struct {
	ID        string
	RepeatKey string
}

var memeCatalogByCategory = map[string][]memeDefinition{
	"FORKPIN": {
		{ID: "meme-fork-pin-0c0f6e16c49a0ce386c57e3aa53f6846", RepeatKey: "0c0f6e16c49a0ce386c57e3aa53f6846"},
		{ID: "meme-fork-pin-20a1154f3299af7c127baa74a138eec3-720w", RepeatKey: "20a1154f3299af7c127baa74a138eec3-720w"},
		{ID: "meme-fork-pin-35958497019901600", RepeatKey: "35958497019901600"},
		{ID: "meme-fork-pin-36169603253819179", RepeatKey: "36169603253819179"},
		{ID: "meme-fork-pin-64b1a33245f6a9ab05741e44c12f08d5", RepeatKey: "64b1a33245f6a9ab05741e44c12f08d5"},
		{ID: "meme-fork-pin-7a5e3c3da6b620b49ceda967f238410f", RepeatKey: "7a5e3c3da6b620b49ceda967f238410f"},
		{ID: "meme-fork-pin-9218374232068722", RepeatKey: "9218374232068722"},
		{ID: "meme-fork-pin-adc9fefd93cd84cdc36e7802aaace891-720w", RepeatKey: "adc9fefd93cd84cdc36e7802aaace891-720w"},
		{ID: "meme-fork-pin-chuvaaak-scary-movie", RepeatKey: "chuvaaak-scary-movie"},
		{ID: "meme-fork-pin-surprise-mazafaka", RepeatKey: "surprise-mazafaka"},
		{ID: "meme-fork-pin-white-cat-dance-mem", RepeatKey: "white-cat-dance-mem"},
		{ID: "meme-fork-pin-yvaunichtuzhu", RepeatKey: "yvaunichtuzhu"},
	},
	"RAZVITIEFIGURI": {
		{ID: "meme-development-01f688eadc26b6769774842ed9c32a17-720w", RepeatKey: "01f688eadc26b6769774842ed9c32a17-720w"},
		{ID: "meme-development-22377329392007237", RepeatKey: "22377329392007237"},
		{ID: "meme-development-3d2283d71abc1d7cebb6fe36c9aec1f0", RepeatKey: "3d2283d71abc1d7cebb6fe36c9aec1f0"},
		{ID: "meme-development-456e21150d0987dd6542a47da2b81908-720w", RepeatKey: "456e21150d0987dd6542a47da2b81908-720w"},
		{ID: "meme-development-554dc6b6a499fc8df9097ca877532100", RepeatKey: "554dc6b6a499fc8df9097ca877532100"},
		{ID: "meme-development-8615cb78db0b9dfcadeb294f9e09b65b", RepeatKey: "8615cb78db0b9dfcadeb294f9e09b65b"},
		{ID: "meme-development-8d015806f6eae3b7d409dd8eae55b6df-720w", RepeatKey: "8d015806f6eae3b7d409dd8eae55b6df-720w"},
		{ID: "meme-development-93bdaec9d3a21d9a5154273acc9cf62e-360w", RepeatKey: "93bdaec9d3a21d9a5154273acc9cf62e-360w"},
		{ID: "meme-development-c9d67e7156a132cf3dfdd5c3e76b76fd-720w", RepeatKey: "c9d67e7156a132cf3dfdd5c3e76b76fd-720w"},
		{ID: "meme-development-ed3c6a50a66a99655f2c18a2ae785e7f", RepeatKey: "ed3c6a50a66a99655f2c18a2ae785e7f"},
		{ID: "meme-development-nobrain", RepeatKey: "nobrain"},
		{ID: "meme-development-polskaya-korova", RepeatKey: "polskaya-korova"},
		{ID: "meme-development-toyota", RepeatKey: "toyota"},
		{ID: "meme-development-user-8vuij-bfmjoi3u7u", RepeatKey: "user-8vuij-bfmjoi3u7u"},
		{ID: "meme-development-uz-multa-idut", RepeatKey: "uz-multa-idut"},
	},
	"SHAH": {
		{ID: "meme-check-157d9427b1438a75430e4c981a0dcfba", RepeatKey: "157d9427b1438a75430e4c981a0dcfba"},
		{ID: "meme-check-56ece945da8e859a25ddcc7f4cab3419", RepeatKey: "56ece945da8e859a25ddcc7f4cab3419"},
		{ID: "meme-check-abbb1893603c4988f1d3c0c0862520a0", RepeatKey: "abbb1893603c4988f1d3c0c0862520a0"},
		{ID: "meme-check-d3c718e8bd76eeeee50cd21c6c5a77e8-720w", RepeatKey: "d3c718e8bd76eeeee50cd21c6c5a77e8-720w"},
		{ID: "meme-check-smile-face", RepeatKey: "smile-face"},
		{ID: "meme-check-suslik", RepeatKey: "suslik"},
		{ID: "meme-check-asset-7", RepeatKey: "asset-7"},
	},
	"VZYATIEVAZHNOIFIGYRI": {
		{ID: "meme-important-capture-2f1fbf894f7a911d457d7fad77fc6b2d-720w", RepeatKey: "2f1fbf894f7a911d457d7fad77fc6b2d-720w"},
		{ID: "meme-important-capture-53761789298088161", RepeatKey: "53761789298088161"},
		{ID: "meme-important-capture-587227238941756156", RepeatKey: "587227238941756156"},
		{ID: "meme-important-capture-nice", RepeatKey: "nice"},
		{ID: "meme-important-capture-a-lovko-ty-eto-pridumal", RepeatKey: "a-lovko-ty-eto-pridumal"},
		{ID: "meme-important-capture-a3abc1b4f2f10d2d64925febfc6bee1f", RepeatKey: "a3abc1b4f2f10d2d64925febfc6bee1f"},
		{ID: "meme-important-capture-aa2620d89a3fb75eb40635b27e834a59", RepeatKey: "aa2620d89a3fb75eb40635b27e834a59"},
		{ID: "meme-important-capture-b63f5c77e3be66c63856c8c14de3d64e-720w", RepeatKey: "b63f5c77e3be66c63856c8c14de3d64e-720w"},
		{ID: "meme-important-capture-d47c8bbd5b90616ae080f054fd520d53-720w", RepeatKey: "d47c8bbd5b90616ae080f054fd520d53-720w"},
		{ID: "meme-important-capture-uuuuuuuuu", RepeatKey: "uuuuuuuuu"},
		{ID: "meme-important-capture-vlipsy-street-fighter-yes-viirpc45", RepeatKey: "vlipsy-street-fighter-yes-viirpc45"},
		{ID: "meme-important-capture-asset-12", RepeatKey: "asset-12"},
	},
	"ZHERTVA": {
		{ID: "meme-sacrifice-18014467258694274", RepeatKey: "18014467258694274"},
		{ID: "meme-sacrifice-32228953578585294", RepeatKey: "32228953578585294"},
		{ID: "meme-sacrifice-587297607708268785", RepeatKey: "587297607708268785"},
		{ID: "meme-sacrifice-63f8452d5bd24fbf5dc550c18c0c898e", RepeatKey: "63f8452d5bd24fbf5dc550c18c0c898e"},
		{ID: "meme-sacrifice-8c11df5a17dd9db50f3a485dd0282af6", RepeatKey: "8c11df5a17dd9db50f3a485dd0282af6"},
		{ID: "meme-sacrifice-91549804920230485", RepeatKey: "91549804920230485"},
		{ID: "meme-sacrifice-xl6edmommbtl", RepeatKey: "xl6edmommbtl"},
		{ID: "meme-sacrifice-best-cry-ever-1", RepeatKey: "best-cry-ever-1"},
		{ID: "meme-sacrifice-efb653bc76527640883deef41a655717-720w", RepeatKey: "efb653bc76527640883deef41a655717-720w"},
		{ID: "meme-sacrifice-f7f526a6c9947d730479fcccb9655351-720w", RepeatKey: "f7f526a6c9947d730479fcccb9655351-720w"},
		{ID: "meme-sacrifice-ishowspeed", RepeatKey: "ishowspeed"},
		{ID: "meme-sacrifice-somnenie-okey-tinkov", RepeatKey: "somnenie-okey-tinkov"},
		{ID: "meme-sacrifice-ty-po-moemu-pereputal", RepeatKey: "ty-po-moemu-pereputal"},
		{ID: "meme-sacrifice-vlipsy-naked-gun-33-13-forehead-slap-ai8nvyjk", RepeatKey: "vlipsy-naked-gun-33-13-forehead-slap-ai8nvyjk"},
		{ID: "meme-sacrifice-vlipsy-vine-black-guy-disappears-waph5eqm", RepeatKey: "vlipsy-vine-black-guy-disappears-waph5eqm"},
		{ID: "meme-sacrifice-ya-poshutil-ili-net-meme-film", RepeatKey: "ya-poshutil-ili-net-meme-film"},
		{ID: "meme-sacrifice-asset-17", RepeatKey: "asset-17"},
	},
}

var memeCatalogByID = buildMemeCatalogIndex()

func buildMemeCatalogIndex() map[string]memeDefinition {
	index := make(map[string]memeDefinition, 64)
	for _, definitions := range memeCatalogByCategory {
		for _, definition := range definitions {
			index[definition.ID] = definition
		}
	}
	return index
}

func normalizeMemeCategory(category string) string {
	normalized := strings.TrimSpace(strings.ToUpper(category))
	if _, ok := memeCatalogByCategory[normalized]; ok {
		return normalized
	}
	return memeCategoryDevelopment
}

func isKnownMemeCategory(category string) bool {
	normalized := strings.TrimSpace(strings.ToUpper(category))
	_, ok := memeCatalogByCategory[normalized]
	return ok
}

func isKnownMemeID(id string) bool {
	normalized := strings.TrimSpace(id)
	if normalized == "" {
		return false
	}
	_, ok := memeCatalogByID[normalized]
	return ok
}

func isKnownMemeAssignment(id string, category string) bool {
	normalizedID := strings.TrimSpace(id)
	normalizedCategory := strings.TrimSpace(strings.ToUpper(category))
	if normalizedID == "" || normalizedCategory == "" {
		return false
	}

	definitions, ok := memeCatalogByCategory[normalizedCategory]
	if !ok {
		return false
	}

	for _, definition := range definitions {
		if definition.ID == normalizedID {
			return true
		}
	}

	return false
}

func classifyMoveMemeCategory(gameMode string, result MoveResult, analysisResult *analysis.Result) string {
	return classifyMoveMemeCategoryWithClassicPosition(gameMode, result, analysisResult, nil)
}

func classifyMoveMemeCategoryWithClassicPosition(
	gameMode string,
	result MoveResult,
	analysisResult *analysis.Result,
	classicPosition *position.GameState,
) string {
	mode := normalizeGameMode(gameMode)
	if mode == GameModeClassic || mode == GameModeMeme {
		return classifyClassicMoveMemeCategoryWithPosition(result, analysisResult, classicPosition)
	}
	return classifyFallbackMoveMemeCategory(result)
}

func classifyClassicMoveMemeCategory(result MoveResult, analysisResult *analysis.Result) string {
	return classifyClassicMoveMemeCategoryWithPosition(result, analysisResult, nil)
}

func classifyClassicMoveMemeCategoryWithPosition(
	result MoveResult,
	analysisResult *analysis.Result,
	classicPosition *position.GameState,
) string {
	if detectsClassicSacrifice(classicPosition) {
		return memeCategorySacrifice
	}

	if analysisResult != nil {
		if result.IsCheck || hasAnyTag(
			analysisResult.Tags,
			pattern.TagCheck,
			pattern.TagDoubleCheck,
			pattern.TagPerpetualCheck,
			pattern.TagCastlingCheck,
			pattern.TagCheckmate,
			pattern.TagForcedMate,
		) {
			return memeCategoryCheck
		}
		if hasAnyTag(
			analysisResult.Tags,
			pattern.TagFork,
			pattern.TagPinToKing,
			pattern.TagRelativePin,
		) {
			return memeCategoryForkPin
		}
		if result.IsCapture && (hasAnyTag(analysisResult.Tags, pattern.TagWinMaterial, pattern.TagConversion) ||
			isSolidMoveQuality(analysisResult.Quality)) {
			return memeCategoryImportantCapture
		}
	}

	return classifyFallbackMoveMemeCategory(result)
}

func classifyFallbackMoveMemeCategory(result MoveResult) string {
	switch {
	case result.IsCheckmate, result.IsCheck:
		return memeCategoryCheck
	case result.IsCapture:
		return memeCategoryImportantCapture
	default:
		return memeCategoryDevelopment
	}
}

func isSolidMoveQuality(quality string) bool {
	switch strings.TrimSpace(strings.ToLower(quality)) {
	case "", "best", "good":
		return true
	default:
		return false
	}
}

func hasAnyTag(tags []pattern.Tag, targets ...pattern.Tag) bool {
	for _, tag := range tags {
		for _, target := range targets {
			if tag == target {
				return true
			}
		}
	}
	return false
}

func selectMoveMeme(gameID, move string, moveNumber int, category string, moves []Move) memeAssignment {
	normalizedCategory := normalizeMemeCategory(category)
	definitions := memeCatalogByCategory[normalizedCategory]
	if len(definitions) == 0 {
		return memeAssignment{}
	}

	previousMoves := moves
	if len(previousMoves) > 0 {
		previousMoves = previousMoves[:len(previousMoves)-1]
	}

	lastMemeID := lastAssignedMemeID(previousMoves)
	recentRepeatKeys := recentRepeatKeys(previousMoves, memeRecentRepeatWindow)

	candidates := filterMemeDefinitions(definitions, func(def memeDefinition) bool {
		return def.ID != lastMemeID && !recentRepeatKeys[def.RepeatKey]
	})
	if len(candidates) == 0 {
		candidates = filterMemeDefinitions(definitions, func(def memeDefinition) bool {
			return def.ID != lastMemeID
		})
	}
	if len(candidates) == 0 {
		candidates = definitions
	}
	candidates = leastUsedMemeDefinitions(candidates, previousMoves)
	if len(candidates) == 0 {
		candidates = definitions
	}

	index := stableMemeIndex(gameID, move, moveNumber, normalizedCategory, len(candidates))
	if index < 0 || index >= len(candidates) {
		index = 0
	}

	return memeAssignment{
		ID:       candidates[index].ID,
		Category: normalizedCategory,
	}
}

func filterMemeDefinitions(definitions []memeDefinition, keep func(memeDefinition) bool) []memeDefinition {
	filtered := make([]memeDefinition, 0, len(definitions))
	for _, definition := range definitions {
		if keep(definition) {
			filtered = append(filtered, definition)
		}
	}
	return filtered
}

func leastUsedMemeDefinitions(definitions []memeDefinition, moves []Move) []memeDefinition {
	if len(definitions) <= 1 {
		return definitions
	}

	usageByID := memeUsageByID(moves)
	minUsage := -1
	filtered := make([]memeDefinition, 0, len(definitions))
	for _, definition := range definitions {
		usage := usageByID[definition.ID]
		if minUsage == -1 || usage < minUsage {
			minUsage = usage
			filtered = filtered[:0]
			filtered = append(filtered, definition)
			continue
		}
		if usage == minUsage {
			filtered = append(filtered, definition)
		}
	}

	if len(filtered) == 0 {
		return definitions
	}
	return filtered
}

func stableMemeIndex(gameID, move string, moveNumber int, category string, size int) int {
	if size <= 1 {
		return 0
	}

	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(strings.TrimSpace(gameID)))
	_, _ = hasher.Write([]byte{'|'})
	_, _ = hasher.Write([]byte(strings.TrimSpace(strings.ToLower(move))))
	_, _ = hasher.Write([]byte{'|'})
	_, _ = hasher.Write([]byte(strings.TrimSpace(strings.ToUpper(category))))
	_, _ = hasher.Write([]byte{'|'})
	_, _ = hasher.Write([]byte(strconv.Itoa(moveNumber)))

	return int(hasher.Sum64() % uint64(size))
}

func lastAssignedMemeID(moves []Move) string {
	for i := len(moves) - 1; i >= 0; i-- {
		if id := strings.TrimSpace(moves[i].MemeID); id != "" {
			return id
		}
	}
	return ""
}

func recentRepeatKeys(moves []Move, limit int) map[string]bool {
	keys := make(map[string]bool, limit)
	if limit <= 0 {
		return keys
	}

	collected := 0
	for i := len(moves) - 1; i >= 0 && collected < limit; i-- {
		id := strings.TrimSpace(moves[i].MemeID)
		if id == "" {
			continue
		}

		definition, ok := memeCatalogByID[id]
		if !ok || strings.TrimSpace(definition.RepeatKey) == "" {
			continue
		}

		keys[definition.RepeatKey] = true
		collected++
	}

	return keys
}

func memeUsageByID(moves []Move) map[string]int {
	usage := make(map[string]int, len(moves))
	for _, move := range moves {
		id := strings.TrimSpace(move.MemeID)
		if id == "" {
			continue
		}
		usage[id]++
	}
	return usage
}
