package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"incident-room-backend/internal/user"
)

// CookieName is the session cookie carrying the opaque token.
const CookieName = "session"

// Authenticator resolves a session token to a user.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (user.User, error)
}

type ctxKey struct{}

// UserFromContext returns the authenticated user set by RequireAuth.
func UserFromContext(ctx context.Context) (user.User, bool) {
	u, ok := ctx.Value(ctxKey{}).(user.User)
	return u, ok
}

// RequireAuth answers 401 without side effects when there is no valid session (BR-10).
// Unexpected authenticator failures answer 500 and are logged with logger, never exposed.
func RequireAuth(authn Authenticator, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(CookieName)
			if err != nil {
				http.Error(w, "unauthenticated", http.StatusUnauthorized)
				return
			}
			u, err := authn.Authenticate(r.Context(), c.Value)
			if err != nil {
				status := http.StatusUnauthorized
				if !errors.Is(err, ErrUnauthenticated) {
					logger.Error("internal error", "err", err)
					status = http.StatusInternalServerError
				}
				http.Error(w, http.StatusText(status), status)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
		})
	}
}

// RequireRole answers 401 without a user in context and 403 when the role is not allowed.
func RequireRole(roles ...user.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := UserFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthenticated", http.StatusUnauthorized)
				return
			}
			if !slices.Contains(roles, u.Role) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
