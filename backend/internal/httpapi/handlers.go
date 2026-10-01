// Package httpapi exposes the HTTP handlers of the API. Handlers only decode,
// validate format and translate domain errors to HTTP status codes.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"incident-room-backend/internal/auth"
	"incident-room-backend/internal/user"
)

const (
	maxBodyBytes = 1 << 20
	// bcrypt only uses the first 72 bytes of a password.
	maxPasswordBytes = 72
)

// Users is the user behavior the handlers need.
type Users interface {
	Register(ctx context.Context, name, email, password string) (user.User, error)
	ChangeRole(ctx context.Context, actor user.User, targetID string, role user.Role) error
}

// Auth is the authentication behavior the handlers need.
type Auth interface {
	Login(ctx context.Context, email, password string) (string, time.Time, error)
	Authenticate(ctx context.Context, token string) (user.User, error)
}

type handlers struct {
	users Users
	auth  Auth
}

// New builds the API router. Only register and login are public (BR-10).
func New(users Users, authn Auth) http.Handler {
	h := &handlers{users: users, auth: authn}
	requireAuth := auth.RequireAuth(authn)
	requireAdmin := auth.RequireRole(user.RoleAdmin)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/register", h.register)
	mux.HandleFunc("POST /api/auth/login", h.login)
	mux.Handle("GET /api/me", requireAuth(http.HandlerFunc(h.me)))
	mux.Handle("PATCH /api/users/{id}/role", requireAuth(requireAdmin(http.HandlerFunc(h.changeRole))))
	return mux
}

func (h *handlers) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Email) == "" || in.Password == "" || len(in.Password) > maxPasswordBytes {
		writeError(w, http.StatusBadRequest, "name, email and password (max 72 bytes) are required")
		return
	}
	u, err := h.users.Register(r.Context(), strings.TrimSpace(in.Name), in.Email, in.Password)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (h *handlers) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	token, expires, err := h.auth.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		fail(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusOK)
}

func (h *handlers) me(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFromContext(r.Context())
	writeJSON(w, http.StatusOK, u)
}

func (h *handlers) changeRole(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Role user.Role `json:"role"`
	}
	if !decode(w, r, &in) {
		return
	}
	switch in.Role {
	case user.RoleIngeniero, user.RoleOncall, user.RoleAdmin:
	default:
		writeError(w, http.StatusBadRequest, "role must be ingeniero, oncall or admin")
		return
	}
	actor, _ := auth.UserFromContext(r.Context())
	if err := h.users.ChangeRole(r.Context(), actor, r.PathValue("id"), in.Role); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

// fail translates domain errors to HTTP; anything else is a 500 that hides details.
func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, user.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email already registered")
	case errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, "invalid credentials")
	case errors.Is(err, user.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, user.ErrNotFound):
		writeError(w, http.StatusNotFound, "user not found")
	default:
		log.Printf("internal error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
