package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"meme_chess/internal/user"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials       = errors.New("invalid credentials")
	ErrWeakPassword             = errors.New("password must be at least 8 characters")
	ErrInvalidUsername          = errors.New("username must be 3-32 characters, letters, digits, underscore")
	ErrDuplicateUser            = errors.New("username or email already taken")
	ErrMissingToken             = errors.New("missing bearer token")
	ErrInvalidToken             = errors.New("invalid token")
	ErrNoProfileChanges         = errors.New("no profile fields to update")
	ErrAvatarURLTooLong         = errors.New("avatar_url is too long")
	ErrPasswordNotSet           = errors.New("password is not set for this account")
	ErrProfileEmailConflict     = errors.New("cannot set email and clear_email in the same request")
	ErrProfileAvatarURLConflict = errors.New("cannot set avatar_url and clear_avatar_url in the same request")
)

const guestUsernamePrefix = "guest_"

type Service struct {
	users *user.Repository
	jwt   *JWTManager
}

func NewService(users *user.Repository, jwt *JWTManager) *Service {
	return &Service{users: users, jwt: jwt}
}

type RegisterInput struct {
	Username string
	Email    string
	Password string
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (token string, u *user.User, err error) {
	username := strings.TrimSpace(in.Username)
	if !validUsername(username) {
		return "", nil, ErrInvalidUsername
	}
	if len(in.Password) < 8 {
		return "", nil, ErrWeakPassword
	}

	email := strings.TrimSpace(in.Email)
	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", nil, fmt.Errorf("hash password: %w", err)
	}

	id, err := s.users.Create(ctx, username, emailPtr, string(hash))
	if err != nil {
		if isUniqueViolation(err) {
			return "", nil, ErrDuplicateUser
		}
		return "", nil, err
	}

	token, err = s.jwt.Generate(id)
	if err != nil {
		return "", nil, err
	}

	u, err = s.users.GetByID(ctx, id)
	if err != nil {
		return "", nil, err
	}
	if u == nil {
		return "", nil, ErrInvalidToken
	}

	return token, u, nil
}

type LoginInput struct {
	Login    string
	Password string
}

func (s *Service) Login(ctx context.Context, in LoginInput) (token string, u *user.User, err error) {
	login := strings.TrimSpace(in.Login)
	if login == "" || in.Password == "" {
		return "", nil, ErrInvalidCredentials
	}

	u, err = s.users.GetByLogin(ctx, login)
	if err != nil {
		return "", nil, err
	}
	if u == nil || u.PasswordHash == "" {
		return "", nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)); err != nil {
		return "", nil, ErrInvalidCredentials
	}

	token, err = s.jwt.Generate(u.ID)
	if err != nil {
		return "", nil, err
	}
	return token, u, nil
}

func (s *Service) UserFromBearer(ctx context.Context, authorization string) (*user.User, error) {
	raw := strings.TrimSpace(authorization)
	if !strings.HasPrefix(strings.ToLower(raw), "bearer ") {
		return nil, ErrMissingToken
	}
	token := strings.TrimSpace(raw[7:])
	if token == "" {
		return nil, ErrMissingToken
	}

	claims, err := s.jwt.Parse(token)
	if err != nil {
		return nil, ErrInvalidToken
	}

	u, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrInvalidToken
	}
	return u, nil
}

type ProfilePatch struct {
	UsernameSet    bool
	Username       string
	EmailSet       bool
	Email          string
	AvatarURLSet   bool
	AvatarURL      string
	ClearEmail     bool
	ClearAvatarURL bool
}

const maxAvatarURLLen = 2048

func (s *Service) UpdateProfile(ctx context.Context, userID string, p ProfilePatch) (*user.User, error) {
	if !p.UsernameSet && !p.EmailSet && !p.AvatarURLSet && !p.ClearEmail && !p.ClearAvatarURL {
		return nil, ErrNoProfileChanges
	}

	if p.ClearEmail && p.EmailSet {
		return nil, ErrProfileEmailConflict
	}
	if p.ClearAvatarURL && p.AvatarURLSet {
		return nil, ErrProfileAvatarURLConflict
	}

	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrInvalidToken
	}

	username := u.Username
	if p.UsernameSet {
		username = strings.TrimSpace(p.Username)
		if !validUsername(username) {
			return nil, ErrInvalidUsername
		}
	}

	email := u.Email
	if p.ClearEmail {
		email = nil
	} else if p.EmailSet {
		e := strings.TrimSpace(p.Email)
		email = &e
	}

	avatarURL := u.AvatarURL
	if p.ClearAvatarURL {
		avatarURL = nil
	} else if p.AvatarURLSet {
		a := strings.TrimSpace(p.AvatarURL)
		if len(a) > maxAvatarURLLen {
			return nil, ErrAvatarURLTooLong
		}
		avatarURL = &a
	}

	if err := s.users.UpdateProfile(ctx, userID, username, email, avatarURL); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateUser
		}
		return nil, err
	}

	return s.users.GetByID(ctx, userID)
}

type ChangePasswordInput struct {
	CurrentPassword string
	NewPassword     string
}

func (s *Service) ChangePassword(ctx context.Context, userID string, in ChangePasswordInput) error {
	if len(in.NewPassword) < 8 {
		return ErrWeakPassword
	}

	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if u == nil {
		return ErrInvalidToken
	}
	if u.PasswordHash == "" {
		return ErrPasswordNotSet
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.CurrentPassword)); err != nil {
		return ErrInvalidCredentials
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	return s.users.UpdatePasswordHash(ctx, userID, string(hash))
}

func (s *Service) Logout(authorization string) error {
	raw := strings.TrimSpace(authorization)
	if !strings.HasPrefix(strings.ToLower(raw), "bearer ") {
		return ErrMissingToken
	}

	token := strings.TrimSpace(raw[7:])
	if token == "" {
		return ErrMissingToken
	}

	if err := s.jwt.Revoke(token); err != nil {
		return ErrInvalidToken
	}

	return nil
}

func (s *Service) CreateGuestSession(ctx context.Context) (token string, u *user.User, err error) {
	for attempt := 0; attempt < 5; attempt++ {
		username, err := newGuestUsername()
		if err != nil {
			return "", nil, err
		}

		id, err := s.users.Create(ctx, username, nil, "")
		if err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return "", nil, err
		}

		token, err = s.jwt.Generate(id)
		if err != nil {
			return "", nil, err
		}

		u, err = s.users.GetByID(ctx, id)
		if err != nil {
			return "", nil, err
		}
		if u == nil {
			return "", nil, ErrInvalidToken
		}

		return token, u, nil
	}

	return "", nil, errors.New("failed to create guest session")
}

func IsGuestUsername(username string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(username)), guestUsernamePrefix)
}

func validUsername(s string) bool {
	if len(s) < 3 || len(s) > 32 {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func newGuestUsername() (string, error) {
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("generate guest username: %w", err)
	}

	return guestUsernamePrefix + hex.EncodeToString(suffix[:]), nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
