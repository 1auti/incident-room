// Package httpapi exposes the HTTP handlers of the API. Handlers only decode,
// validate format and translate domain errors to HTTP status codes.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"incident-room-backend/internal/auth"
	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/service"
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

// Services is the service catalog behavior the handlers need.
type Services interface {
	List(ctx context.Context) ([]service.Service, error)
	Create(ctx context.Context, actor user.User, name string, criticality incident.Criticality) (service.Service, error)
	Update(ctx context.Context, actor user.User, id, name string, criticality incident.Criticality) (service.Service, error)
	Delete(ctx context.Context, actor user.User, id string) error
}

// Incidents is the incident behavior the handlers need.
type Incidents interface {
	Suggest(ctx context.Context, actor user.User, serviceID string, impact incident.Impact) (incident.Severity, error)
	Declare(ctx context.Context, actor user.User, in incident.DeclareInput) (incident.Incident, []incident.TimelineEvent, error)
	List(ctx context.Context, actor user.User, f incident.ListFilter) ([]incident.Incident, error)
}

type handlers struct {
	users     Users
	auth      Auth
	services  Services
	incidents Incidents
	logger    *slog.Logger
}

// New builds the API router. Only register and login are public (BR-10).
func New(users Users, authn Auth, services Services, incidents Incidents, logger *slog.Logger) http.Handler {
	h := &handlers{users: users, auth: authn, services: services, incidents: incidents, logger: logger}
	requireAuth := auth.RequireAuth(authn, logger)
	requireAdmin := auth.RequireRole(user.RoleAdmin)

	mux := http.NewServeMux()
	mux.Handle("GET /api/services", requireAuth(http.HandlerFunc(h.listServices)))
	mux.Handle("POST /api/services", requireAuth(requireAdmin(http.HandlerFunc(h.createService))))
	mux.Handle("PUT /api/services/{id}", requireAuth(requireAdmin(http.HandlerFunc(h.updateService))))
	mux.Handle("DELETE /api/services/{id}", requireAuth(requireAdmin(http.HandlerFunc(h.deleteService))))
	// Every role may declare (BR-10): the service enforces it, so no RequireRole here.
	mux.Handle("GET /api/incidents/suggested-severity", requireAuth(http.HandlerFunc(h.suggestSeverity)))
	mux.Handle("POST /api/incidents", requireAuth(http.HandlerFunc(h.declareIncident)))
	// Every authenticated user sees every active incident (BR-10): no RequireRole.
	mux.Handle("GET /api/incidents", requireAuth(http.HandlerFunc(h.listIncidents)))
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
		h.fail(w, err)
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
		h.fail(w, err)
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
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *handlers) listServices(w http.ResponseWriter, r *http.Request) {
	list, err := h.services.List(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	if list == nil {
		list = []service.Service{}
	}
	writeJSON(w, http.StatusOK, list)
}

type serviceInput struct {
	Name        string               `json:"name"`
	Criticality incident.Criticality `json:"criticality"`
}

func (h *handlers) createService(w http.ResponseWriter, r *http.Request) {
	var in serviceInput
	if !decode(w, r, &in) {
		return
	}
	actor, _ := auth.UserFromContext(r.Context())
	s, err := h.services.Create(r.Context(), actor, in.Name, in.Criticality)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

func (h *handlers) updateService(w http.ResponseWriter, r *http.Request) {
	var in serviceInput
	if !decode(w, r, &in) {
		return
	}
	actor, _ := auth.UserFromContext(r.Context())
	s, err := h.services.Update(r.Context(), actor, r.PathValue("id"), in.Name, in.Criticality)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *handlers) deleteService(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.UserFromContext(r.Context())
	if err := h.services.Delete(r.Context(), actor, r.PathValue("id")); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) suggestSeverity(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.UserFromContext(r.Context())
	q := r.URL.Query()
	sev, err := h.incidents.Suggest(r.Context(), actor, q.Get("service_id"), incident.Impact(q.Get("impact")))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"suggested_severity": string(sev)})
}

// declareIncident ignores any declared_by in the body: the declarer is the
// session user.
func (h *handlers) declareIncident(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title       string            `json:"title"`
		Description string            `json:"description"`
		ServiceID   string            `json:"service_id"`
		Impact      incident.Impact   `json:"impact"`
		Severity    incident.Severity `json:"severity"`
	}
	if !decode(w, r, &in) {
		return
	}
	actor, _ := auth.UserFromContext(r.Context())
	inc, events, err := h.incidents.Declare(r.Context(), actor, incident.DeclareInput{
		Title: in.Title, Description: in.Description, ServiceID: in.ServiceID, Impact: in.Impact, Severity: in.Severity,
	})
	if err != nil {
		h.fail(w, err)
		return
	}
	if events == nil {
		events = []incident.TimelineEvent{}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"incident": inc, "timeline": events})
}

// listIncidents takes at most one value per filter; empty means no filter.
func (h *handlers) listIncidents(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.UserFromContext(r.Context())
	q := r.URL.Query()
	list, err := h.incidents.List(r.Context(), actor, incident.ListFilter{
		Severity:  incident.Severity(q.Get("severity")),
		ServiceID: q.Get("service_id"),
		State:     incident.State(q.Get("state")),
	})
	if err != nil {
		h.fail(w, err)
		return
	}
	if list == nil {
		list = []incident.Incident{}
	}
	writeJSON(w, http.StatusOK, list)
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
func (h *handlers) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid service: name is required and criticality must be critica, importante or estandar")
	case errors.Is(err, service.ErrNameTaken):
		writeError(w, http.StatusConflict, "service name already in use")
	case errors.Is(err, service.ErrInUse):
		writeError(w, http.StatusConflict, "service has incidents or runbooks")
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "service not found")
	case errors.Is(err, service.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, incident.ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid incident: title, service_id and impact (caida_total, degradacion or menor) are required and severity must be SEV1, SEV2 or SEV3")
	case errors.Is(err, incident.ErrInvalidFilter):
		writeError(w, http.StatusBadRequest, "invalid filter: severity must be SEV1, SEV2 or SEV3, state must be declarado, reconocido, mitigando, resuelto or cerrado and service_id must be a valid id")
	case errors.Is(err, incident.ErrServiceNotFound):
		writeError(w, http.StatusBadRequest, "invalid incident: service does not exist")
	case errors.Is(err, incident.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, user.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email already registered")
	case errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, "invalid credentials")
	case errors.Is(err, user.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, user.ErrNotFound):
		writeError(w, http.StatusNotFound, "user not found")
	default:
		h.logger.Error("internal error", "err", err)
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
