package shop

import (
	"context"
	"errors"
	"strings"

	"meme_chess/internal/user"
)

var (
	ErrInvalidAmount = errors.New("invalid amount")
)

type Service struct {
	repo  *Repository
	users *user.Repository
}

func NewService(repo *Repository, users *user.Repository) *Service {
	return &Service{repo: repo, users: users}
}

func (s *Service) GetCatalog(ctx context.Context, userID string) ([]CatalogItem, error) {
	return s.repo.GetCatalog(ctx, userID)
}

func (s *Service) ConvertGameToShop(ctx context.Context, userID string, amount int64) (Currency, error) {
	if amount <= 0 {
		return Currency{}, ErrInvalidAmount
	}
	shopFunds, gameFunds, err := s.users.ConvertGameToShop1to1(ctx, userID, amount)
	if err != nil {
		return Currency{}, err
	}
	return Currency{ShopFunds: shopFunds, GameFunds: gameFunds}, nil
}

func (s *Service) Buy(ctx context.Context, userID string, itemSlug string) (Currency, error) {
	itemSlug = strings.TrimSpace(itemSlug)
	if itemSlug == "" {
		return Currency{}, ErrItemNotForSale
	}

	shopFunds, gameFunds, _, err := s.repo.Buy(ctx, userID, itemSlug)
	if err != nil {
		return Currency{}, err
	}
	return Currency{ShopFunds: shopFunds, GameFunds: gameFunds}, nil
}

