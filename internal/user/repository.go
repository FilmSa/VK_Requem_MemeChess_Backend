package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type User struct {
	ID           string
	Email        *string
	Username     string
	AvatarURL    *string
	ShopCurrency int64
	GameCurrency int64
	CreatedAt    time.Time
	PasswordHash string
}

func (r *Repository) Create(ctx context.Context, username string, email *string, passwordHash string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		INSERT INTO users (id, username, email, password_hash, shop_currency, game_currency)
		VALUES (gen_random_uuid(), $1, $2, $3, 0, 1000)
		RETURNING id::text
	`

	var id string
	err := r.pool.QueryRow(ctx, q, username, email, passwordHash).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert user: %w", err)
	}
	return id, nil
}

func (r *Repository) GetByLogin(ctx context.Context, login string) (*User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		SELECT id::text, email, username, avatar_url, shop_currency, game_currency, created_at, password_hash
		FROM users
		WHERE lower(username) = lower($1)
		   OR (email IS NOT NULL AND lower(email) = lower($1))
	`

	var u User
	err := r.pool.QueryRow(ctx, q, login).Scan(
		&u.ID,
		&u.Email,
		&u.Username,
		&u.AvatarURL,
		&u.ShopCurrency,
		&u.GameCurrency,
		&u.CreatedAt,
		&u.PasswordHash,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select user: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		SELECT id::text, email, username, avatar_url, shop_currency, game_currency, created_at, password_hash
		FROM users
		WHERE id = $1
	`

	var u User
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&u.ID,
		&u.Email,
		&u.Username,
		&u.AvatarURL,
		&u.ShopCurrency,
		&u.GameCurrency,
		&u.CreatedAt,
		&u.PasswordHash,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select user: %w", err)
	}
	return &u, nil
}

var ErrInsufficientGameCurrency = errors.New("insufficient game currency")
var ErrInsufficientShopCurrency = errors.New("insufficient shop currency")

func (r *Repository) ReserveGameCurrency(ctx context.Context, userID string, amount int64) error {
	if amount <= 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		UPDATE users
		SET game_currency = game_currency - $2
		WHERE id = $1
		  AND game_currency >= $2
	`

	tag, err := r.pool.Exec(ctx, q, userID, amount)
	if err != nil {
		return fmt.Errorf("reserve game currency: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrInsufficientGameCurrency
	}
	return nil
}

func (r *Repository) AddGameCurrency(ctx context.Context, userID string, amount int64) error {
	if amount <= 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		UPDATE users
		SET game_currency = game_currency + $2
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, q, userID, amount)
	if err != nil {
		return fmt.Errorf("add game currency: %w", err)
	}
	return nil
}

func (r *Repository) GetCurrencies(ctx context.Context, userID string) (shop int64, game int64, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		SELECT shop_currency, game_currency
		FROM users
		WHERE id = $1
	`

	err = r.pool.QueryRow(ctx, q, userID).Scan(&shop, &game)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("select currencies: %w", err)
	}
	return shop, game, nil
}

func (r *Repository) ReserveShopCurrency(ctx context.Context, userID string, amount int64) error {
	if amount <= 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		UPDATE users
		SET shop_currency = shop_currency - $2
		WHERE id = $1
		  AND shop_currency >= $2
	`

	tag, err := r.pool.Exec(ctx, q, userID, amount)
	if err != nil {
		return fmt.Errorf("reserve shop currency: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrInsufficientShopCurrency
	}
	return nil
}

func (r *Repository) AddShopCurrency(ctx context.Context, userID string, amount int64) error {
	if amount <= 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		UPDATE users
		SET shop_currency = shop_currency + $2
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, q, userID, amount)
	if err != nil {
		return fmt.Errorf("add shop currency: %w", err)
	}
	return nil
}

func (r *Repository) ConvertGameToShop1to1(ctx context.Context, userID string, amount int64) (shop int64, game int64, err error) {
	if amount <= 0 {
		return r.GetCurrencies(ctx, userID)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		UPDATE users
		SET game_currency = game_currency - $2,
		    shop_currency = shop_currency + $2
		WHERE id = $1
		  AND game_currency >= $2
		RETURNING shop_currency, game_currency
	`

	err = r.pool.QueryRow(ctx, q, userID, amount).Scan(&shop, &game)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, ErrInsufficientGameCurrency
		}
		return 0, 0, fmt.Errorf("convert game->shop: %w", err)
	}
	return shop, game, nil
}

var ErrUserNotFound = errors.New("user not found")

func (r *Repository) UpdateProfile(ctx context.Context, userID string, username string, email *string, avatarURL *string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		UPDATE users
		SET username = $2, email = $3, avatar_url = $4
		WHERE id = $1
	`

	tag, err := r.pool.Exec(ctx, q, userID, username, email, avatarURL)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *Repository) UpdatePasswordHash(ctx context.Context, userID string, passwordHash string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const q = `
		UPDATE users
		SET password_hash = $2
		WHERE id = $1
	`

	tag, err := r.pool.Exec(ctx, q, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}
