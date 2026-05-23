package inventory

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrTooManyEmotes    = errors.New("too many emotes selected (max 3)")
	ErrDuplicateEmotes  = errors.New("duplicate emotes are not allowed")
	ErrItemNotOwned     = errors.New("item is not owned")
	ErrItemNotFound     = errors.New("item not found")
	ErrInvalidItemType  = errors.New("invalid item type")
	ErrEmoteNotSelected = errors.New("emote is not selected")
)

var (
	ErrTooManyStickers    = ErrTooManyEmotes
	ErrDuplicateStickers  = ErrDuplicateEmotes
	ErrStickerNotSelected = ErrEmoteNotSelected
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetCatalog(ctx context.Context) ([]Item, error) {
	return s.repo.GetCatalog(ctx)
}

func (s *Service) GetInventory(ctx context.Context, userID string) (Inventory, error) {
	owned, err := s.repo.GetOwned(ctx, userID)
	if err != nil {
		return Inventory{}, err
	}
	sel, err := s.repo.GetSelection(ctx, userID)
	if err != nil {
		return Inventory{}, err
	}
	return Inventory{
		Owned:    owned,
		Selected: sel,
	}, nil
}

func (s *Service) GetSelection(ctx context.Context, userID string) (Selection, error) {
	return s.repo.GetSelection(ctx, userID)
}

func (s *Service) SetSelection(ctx context.Context, userID string, sel Selection) (Selection, error) {
	normalized, err := s.normalizeSelection(sel)
	if err != nil {
		return Selection{}, err
	}
	if err := s.validateSelectionOwnedAndTyped(ctx, userID, normalized); err != nil {
		return Selection{}, err
	}
	return s.repo.SetSelection(ctx, userID, normalized)
}

func (s *Service) ResolveSelectedEmoteAssetURL(ctx context.Context, userID string, emoteSlug string) (string, error) {
	emoteSlug = strings.TrimSpace(emoteSlug)
	if emoteSlug == "" {
		return "", ErrItemNotFound
	}

	sel, err := s.repo.GetSelection(ctx, userID)
	if err != nil {
		return "", err
	}
	if !contains(sel.EmoteSlugs, emoteSlug) {
		return "", ErrEmoteNotSelected
	}

	ok, err := s.repo.UserOwns(ctx, userID, emoteSlug)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrItemNotOwned
	}

	it, err := s.repo.GetItem(ctx, emoteSlug)
	if err != nil {
		return "", err
	}
	if it == nil {
		return "", ErrItemNotFound
	}
	if it.Type != ItemTypeEmote {
		return "", ErrInvalidItemType
	}
	if it.AssetURL == nil || strings.TrimSpace(*it.AssetURL) == "" {
		return "", fmt.Errorf("emote has no asset_url")
	}
	return strings.TrimSpace(*it.AssetURL), nil
}

func (s *Service) ResolveSelectedStickerAssetURL(ctx context.Context, userID string, stickerSlug string) (string, error) {
	return s.ResolveSelectedEmoteAssetURL(ctx, userID, stickerSlug)
}

func (s *Service) CanSendSelectedEmoteAssetURL(ctx context.Context, userID string, assetURL string) (bool, error) {
	assetURL = strings.TrimSpace(assetURL)
	if assetURL == "" {
		return false, nil
	}
	return s.repo.HasSelectedEmoteAssetURL(ctx, userID, assetURL)
}

func (s *Service) CanSendSelectedStickerAssetURL(ctx context.Context, userID string, assetURL string) (bool, error) {
	return s.CanSendSelectedEmoteAssetURL(ctx, userID, assetURL)
}

func (s *Service) normalizeSelection(sel Selection) (Selection, error) {
	var out Selection
	if sel.PieceSkinSlug != nil {
		v := strings.TrimSpace(*sel.PieceSkinSlug)
		if v == "" {
			out.PieceSkinSlug = nil
		} else {
			out.PieceSkinSlug = &v
		}
	} else {
		out.PieceSkinSlug = nil
	}

	if sel.BoardSkinSlug != nil {
		v := strings.TrimSpace(*sel.BoardSkinSlug)
		if v == "" {
			out.BoardSkinSlug = nil
		} else {
			out.BoardSkinSlug = &v
		}
	} else {
		out.BoardSkinSlug = nil
	}

	seen := make(map[string]struct{})
	for _, raw := range sel.EmoteSlugs {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			return Selection{}, ErrDuplicateEmotes
		}
		seen[v] = struct{}{}
		out.EmoteSlugs = append(out.EmoteSlugs, v)
	}

	if len(out.EmoteSlugs) > 3 {
		return Selection{}, ErrTooManyEmotes
	}
	return out, nil
}

func (s *Service) validateSelectionOwnedAndTyped(ctx context.Context, userID string, sel Selection) error {
	if sel.PieceSkinSlug != nil {
		if err := s.validateItem(ctx, userID, *sel.PieceSkinSlug, ItemTypePieceSkin); err != nil {
			return fmt.Errorf("piece_skin_slug: %w", err)
		}
	}
	if sel.BoardSkinSlug != nil {
		if err := s.validateItem(ctx, userID, *sel.BoardSkinSlug, ItemTypeBoardSkin); err != nil {
			return fmt.Errorf("board_skin_slug: %w", err)
		}
	}
	for _, slug := range sel.EmoteSlugs {
		if err := s.validateItem(ctx, userID, slug, ItemTypeEmote); err != nil {
			return fmt.Errorf("emote_slugs: %w", err)
		}
	}
	return nil
}

func (s *Service) validateItem(ctx context.Context, userID string, slug string, expected ItemType) error {
	ok, err := s.repo.UserOwns(ctx, userID, slug)
	if err != nil {
		return err
	}
	if !ok {
		return ErrItemNotOwned
	}

	it, err := s.repo.GetItem(ctx, slug)
	if err != nil {
		return err
	}
	if it == nil {
		return ErrItemNotFound
	}
	if it.Type != expected {
		return ErrInvalidItemType
	}
	return nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
