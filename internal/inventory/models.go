package inventory

import "time"

type ItemType string

const (
	ItemTypePieceSkin ItemType = "piece_skin"
	ItemTypeBoardSkin ItemType = "board_skin"
	ItemTypeSticker   ItemType = "sticker"
)

type Item struct {
	Slug      string         `json:"slug"`
	Type      ItemType       `json:"type"`
	Title     *string        `json:"title,omitempty"`
	AssetURL  *string        `json:"asset_url,omitempty"`
	Meta      map[string]any `json:"meta,omitempty"`
	CreatedAt time.Time      `json:"created_at,omitempty"`
}

type Selection struct {
	PieceSkinSlug *string  `json:"piece_skin_slug,omitempty"`
	BoardSkinSlug *string  `json:"board_skin_slug,omitempty"`
	StickerSlugs  []string `json:"sticker_slugs"`
}

type Inventory struct {
	Owned    []Item    `json:"owned"`
	Selected Selection `json:"selected"`
}
