package shop

import "meme_chess/internal/inventory"

type CatalogItem struct {
	Item     inventory.Item `json:"item"`
	Price    int64          `json:"price"`
	IsActive bool           `json:"is_active"`
	Owned    bool           `json:"owned"`
}

type Currency struct {
	ShopFunds int64 `json:"shop_funds"`
	GameFunds int64 `json:"game_funds"`
}

