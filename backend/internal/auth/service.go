// Package auth implements login, session authentication and HTTP guards (BR-10).
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"incident-room-backend/internal/user"
)

// Domain errors translated to HTTP by handlers.
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrSessionNotFound    = errors.New("session not found")
)

// Session is a stored login session. Only the token hash is persisted.
type Session struct {
	TokenHash string
	UserID    string
	ExpiresAt time.Time
}

// UserFinder is the user lookup the Service needs.
type UserFinder interface {
	FindByEmail(ctx context.Context, email string) (user.User, error)
	FindByID(ctx context.Context, id string) (user.User, error)
}

// SessionRepository persists sessions; FindByTokenHash returns ErrSessionNotFound when absent.
type SessionRepository interface {
	Create(ctx context.Context, s Session) error
	FindByTokenHash(ctx context.Context, tokenHash string) (Session, error)
}

// dummyPasswordHash is a valid bcrypt hash (cost bcrypt.DefaultCost, the cost
// user.Service stores) of a random value nobody knows. Login compares against it
// when the email does not exist so both failure paths cost one bcrypt comparison.
const dummyPasswordHash = "$2a$10$0LchmUyP/cGjkWz2WwFUpOI5YxeHWgby6c8rwtDsvTPmPtvC1M43W"

// Service authenticates users and resolves sessions.
type Service struct {
	users    UserFinder
	sessions SessionRepository
	ttl      time.Duration
	now      func() time.Time
}

// NewService builds a Service. now is injected for deterministic expiry.
func NewService(users UserFinder, sessions SessionRepository, ttl time.Duration, now func() time.Time) *Service {
	return &Service{users: users, sessions: sessions, ttl: ttl, now: now}
}

// Login verifies credentials and returns an opaque session token and its expiry.
// Unknown email and wrong password are indistinguishable: same error and, because
// an unknown email still pays one bcrypt comparison against a decoy hash, the same cost.
func (s *Service) Login(ctx context.Context, email, password string) (string, time.Time, error) {
	u, err := s.users.FindByEmail(ctx, user.NormalizeEmail(email))
	if errors.Is(err, user.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(password))
		return "", time.Time{}, ErrInvalidCredentials
	}
	if err != nil {
		return "", time.Time{}, fmt.Errorf("find user: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", time.Time{}, ErrInvalidCredentials
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, fmt.Errorf("generate token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expires := s.now().Add(s.ttl)
	if err := s.sessions.Create(ctx, Session{TokenHash: hashToken(token), UserID: u.ID, ExpiresAt: expires}); err != nil {
		return "", time.Time{}, fmt.Errorf("create session: %w", err)
	}
	return token, expires, nil
}

// Authenticate resolves a session token to its user, or ErrUnauthenticated.
func (s *Service) Authenticate(ctx context.Context, token string) (user.User, error) {
	if token == "" {
		return user.User{}, ErrUnauthenticated
	}
	sess, err := s.sessions.FindByTokenHash(ctx, hashToken(token))
	if errors.Is(err, ErrSessionNotFound) {
		return user.User{}, ErrUnauthenticated
	}
	if err != nil {
		return user.User{}, fmt.Errorf("find session: %w", err)
	}
	if !s.now().Before(sess.ExpiresAt) {
		return user.User{}, ErrUnauthenticated
	}
	u, err := s.users.FindByID(ctx, sess.UserID)
	if errors.Is(err, user.ErrNotFound) {
		return user.User{}, ErrUnauthenticated
	}
	if err != nil {
		return user.User{}, fmt.Errorf("find session user: %w", err)
	}
	return u, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
