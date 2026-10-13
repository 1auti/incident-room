// Package httpapi exposes the HTTP handlers of the API. Handlers only decode,
// validate format and translate domain errors to HTTP status codes.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"incident-room-backend/internal/auth"
	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/metrics"
	"incident-room-backend/internal/service"
	"incident-room-backend/internal/timeline"
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
	ListByRole(ctx context.Context, actor user.User, role user.Role) ([]user.User, error)
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
	SetOncall(ctx context.Context, actor user.User, id, userID string) (service.Service, error)
}

// Incidents is the incident behavior the handlers need.
type Incidents interface {
	Suggest(ctx context.Context, actor user.User, serviceID string, impact incident.Impact) (incident.Severity, error)
	Declare(ctx context.Context, actor user.User, in incident.DeclareInput) (incident.Incident, []incident.TimelineEvent, error)
	List(ctx context.Context, actor user.User, f incident.ListFilter) ([]incident.Incident, error)
	Get(ctx context.Context, actor user.User, id string) (incident.Incident, error)
	Transition(ctx context.Context, actor user.User, id string, to incident.State, rootCause string) (incident.Incident, error)
}

// Timeline is the timeline behavior the handlers need.
type Timeline interface {
	List(ctx context.Context, actor user.User, incidentID string) ([]incident.EventView, error)
	AddNote(ctx context.Context, actor user.User, incidentID, body string) (incident.TimelineEvent, error)
	ChangeSeverity(ctx context.Context, actor user.User, incidentID string, to incident.Severity) (incident.Incident, error)
}

// Metrics is the dashboard behavior the handlers need.
type Metrics interface {
	Dashboard(ctx context.Context, actor user.User, q metrics.Query) (metrics.Dashboard, error)
}

type handlers struct {
	users     Users
	auth      Auth
	services  Services
	incidents Incidents
	timeline  Timeline
	metrics   Metrics
	logger    *slog.Logger
}

// New builds the API router. Only register and login are public (BR-10).
func New(users Users, authn Auth, services Services, incidents Incidents, tl Timeline, m Metrics, logger *slog.Logger) http.Handler {
	h := &handlers{users: users, auth: authn, services: services, incidents: incidents, timeline: tl, metrics: m, logger: logger}
	requireAuth := auth.RequireAuth(authn, logger)
	requireAdmin := auth.RequireRole(user.RoleAdmin)

	mux := http.NewServeMux()
	mux.Handle("GET /api/services", requireAuth(http.HandlerFunc(h.listServices)))
	mux.Handle("POST /api/services", requireAuth(requireAdmin(http.HandlerFunc(h.createService))))
	mux.Handle("PUT /api/services/{id}", requireAuth(requireAdmin(http.HandlerFunc(h.updateService))))
	mux.Handle("DELETE /api/services/{id}", requireAuth(requireAdmin(http.HandlerFunc(h.deleteService))))
	mux.Handle("PUT /api/services/{id}/oncall", requireAuth(requireAdmin(http.HandlerFunc(h.setOncall))))
	// Every role may declare (BR-10): the service enforces it, so no RequireRole here.
	mux.Handle("GET /api/incidents/suggested-severity", requireAuth(http.HandlerFunc(h.suggestSeverity)))
	mux.Handle("POST /api/incidents", requireAuth(http.HandlerFunc(h.declareIncident)))
	// Every authenticated user sees every active incident (BR-10): no RequireRole.
	mux.Handle("GET /api/incidents", requireAuth(http.HandlerFunc(h.listIncidents)))
	mux.Handle("GET /api/incidents/{id}", requireAuth(http.HandlerFunc(h.getIncident)))
	// The service decides who may transition (BR-08, BR-11): no RequireRole here.
	mux.Handle("POST /api/incidents/{id}/transitions", requireAuth(http.HandlerFunc(h.transitionIncident)))
	// Every authenticated user reads timelines and adds notes (BR-10); the
	// service decides who may change the severity (BR-02, BR-11).
	mux.Handle("GET /api/incidents/{id}/timeline", requireAuth(http.HandlerFunc(h.listTimeline)))
	mux.Handle("POST /api/incidents/{id}/notes", requireAuth(http.HandlerFunc(h.addNote)))
	mux.Handle("PATCH /api/incidents/{id}/severity", requireAuth(http.HandlerFunc(h.changeSeverity)))
	// Timeline events are append-only (BR-09): no method may touch a single event.
	mux.Handle("/api/incidents/{id}/timeline/{eventID}", requireAuth(http.HandlerFunc(eventsAreImmutable)))
	// Every authenticated user sees the dashboard (BR-10).
	mux.Handle("GET /api/metrics", requireAuth(http.HandlerFunc(h.dashboard)))
	mux.Handle("GET /api/users", requireAuth(requireAdmin(http.HandlerFunc(h.listUsers))))
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

func (h *handlers) setOncall(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID string `json:"user_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	actor, _ := auth.UserFromContext(r.Context())
	s, err := h.services.SetOncall(r.Context(), actor, r.PathValue("id"), in.UserID)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *handlers) listUsers(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.UserFromContext(r.Context())
	list, err := h.users.ListByRole(r.Context(), actor, user.Role(r.URL.Query().Get("role")))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *handlers) getIncident(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.UserFromContext(r.Context())
	inc, err := h.incidents.Get(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inc)
}

func (h *handlers) transitionIncident(w http.ResponseWriter, r *http.Request) {
	var in struct {
		To        incident.State `json:"to"`
		RootCause string         `json:"root_cause"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !incident.ValidState(in.To) {
		writeError(w, http.StatusBadRequest, "to must be declarado, reconocido, mitigando, resuelto or cerrado")
		return
	}
	actor, _ := auth.UserFromContext(r.Context())
	inc, err := h.incidents.Transition(r.Context(), actor, r.PathValue("id"), in.To, in.RootCause)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inc)
}

func (h *handlers) listTimeline(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.UserFromContext(r.Context())
	events, err := h.timeline.List(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		h.fail(w, err)
		return
	}
	if events == nil {
		events = []incident.EventView{}
	}
	writeJSON(w, http.StatusOK, events)
}

func (h *handlers) addNote(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Body string `json:"body"`
	}
	if !decode(w, r, &in) {
		return
	}
	actor, _ := auth.UserFromContext(r.Context())
	ev, err := h.timeline.AddNote(r.Context(), actor, r.PathValue("id"), in.Body)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ev)
}

func (h *handlers) changeSeverity(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Severity incident.Severity `json:"severity"`
	}
	if !decode(w, r, &in) {
		return
	}
	actor, _ := auth.UserFromContext(r.Context())
	inc, err := h.timeline.ChangeSeverity(r.Context(), actor, r.PathValue("id"), in.Severity)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inc)
}

// eventsAreImmutable answers 405 with an empty Allow: no method is allowed on
// a single timeline event (BR-09).
func eventsAreImmutable(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Allow", "")
	writeError(w, http.StatusMethodNotAllowed, "timeline events are append-only")
}

// dashboard reads ?period=<n>d or a free range ?from=&to= (RFC 3339); with
// neither the service applies its default period. Which presets exist is the
// service's decision (ErrInvalidPeriod).
func (h *handlers) dashboard(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	var q metrics.Query
	if p := params.Get("period"); p != "" {
		digits, ok := strings.CutSuffix(p, "d")
		days, err := strconv.Atoi(digits)
		if !ok || err != nil || days <= 0 {
			writeError(w, http.StatusBadRequest, "period must be 7d or 30d")
			return
		}
		q.LastDays = days
	}
	for _, f := range []struct {
		name string
		dst  *time.Time
	}{{"from", &q.From}, {"to", &q.To}} {
		name, dst := f.name, f.dst
		v := params.Get(name)
		if v == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, name+" must be an RFC 3339 instant")
			return
		}
		*dst = t
	}
	actor, _ := auth.UserFromContext(r.Context())
	d, err := h.metrics.Dashboard(r.Context(), actor, q)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
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
	case errors.Is(err, service.ErrInvalidOncall):
		writeError(w, http.StatusBadRequest, "invalid on-call: user_id must be an existing user with role oncall")
	case errors.Is(err, service.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, incident.ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid incident: title, service_id and impact (caida_total, degradacion or menor) are required and severity must be SEV1, SEV2 or SEV3")
	case errors.Is(err, incident.ErrInvalidFilter):
		writeError(w, http.StatusBadRequest, "invalid filter: severity must be SEV1, SEV2 or SEV3, state must be declarado, reconocido, mitigando, resuelto or cerrado and service_id must be a valid id")
	case errors.Is(err, incident.ErrServiceNotFound):
		writeError(w, http.StatusBadRequest, "invalid incident: service does not exist")
	case errors.Is(err, incident.ErrNotFound):
		writeError(w, http.StatusNotFound, "incident not found")
	case errors.Is(err, incident.ErrInvalidTransition):
		writeError(w, http.StatusConflict, "invalid state transition")
	case errors.Is(err, metrics.ErrInvalidPeriod):
		writeError(w, http.StatusBadRequest, "invalid period: use period=7d|30d or both from and to with from before to, not both")
	case errors.Is(err, incident.ErrRootCauseRequired):
		writeError(w, http.StatusBadRequest, "invalid transition: resolving requires a non-empty root_cause")
	case errors.Is(err, incident.ErrPostmortemRequired):
		writeError(w, http.StatusConflict, "a SEV1 incident can only be closed with an approved postmortem")
	case errors.Is(err, incident.ErrSeverityLocked):
		writeError(w, http.StatusConflict, "severity can no longer change: the incident is resuelto or cerrado, or its severity changed meanwhile")
	case errors.Is(err, timeline.ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid input: a note needs a non-empty body and severity must be SEV1, SEV2 or SEV3 and differ from the current one")
	case errors.Is(err, incident.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, user.ErrOncallAssigned):
		writeError(w, http.StatusConflict, "user is on-call of a service: assign another on-call first")
	case errors.Is(err, user.ErrInvalidRole):
		writeError(w, http.StatusBadRequest, "role must be ingeniero, oncall or admin")
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
