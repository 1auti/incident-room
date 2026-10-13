// Package copilot answers questions about the runbooks (RAG) and runs three
// tools (open an incident, add a note, list the active ones) as the
// authenticated user, through the same services the API uses (BR-10, BR-14,
// BR-15). It never imports the LLM library: Model is implemented in package llm.
package copilot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/runbook"
	"incident-room-backend/internal/service"
	"incident-room-backend/internal/user"
)

// MaxRounds bounds the model -> tools -> model loop of one question.
const MaxRounds = 4

// NoInformation is the fixed answer when no runbook is relevant and no tool
// was used: the copilot does not let the model improvise a procedure (UC-10.2).
// It also states what the tools can and cannot do, because a request for an
// action without a tool ("cerrá el incidente", UC-10.6) lands here too: the
// model declines in plain text, with no runbook and no tool call.
const NoInformation = "No encontré información sobre eso en los runbooks, así que no voy a proponer un procedimiento: " +
	"revisá los runbooks del servicio o consultá al on-call. Con mis herramientas solo puedo abrir incidentes, " +
	"agregar notas y listar los incidentes abiertos; no puedo cerrar, resolver ni transicionar incidentes."

// Domain errors translated to HTTP by handlers.
var (
	ErrForbidden = errors.New("forbidden")
	ErrInvalid   = errors.New("invalid message: it must not be empty")
	// ErrModel: the model failed or did not finish within MaxRounds.
	ErrModel = errors.New("copilot model unavailable")
)

// Retriever finds the runbooks related to a question (package runbook).
type Retriever interface {
	Search(ctx context.Context, actor user.User, query string, k int, minSimilarity float64) ([]runbook.Match, error)
}

// Incidents is what the incident tools call (package incident).
type Incidents interface {
	Declare(ctx context.Context, actor user.User, in incident.DeclareInput) (incident.Incident, []incident.TimelineEvent, error)
	List(ctx context.Context, actor user.User, f incident.ListFilter) ([]incident.Incident, error)
}

// Notes is what the note tool calls (package timeline).
type Notes interface {
	AddNote(ctx context.Context, actor user.User, incidentID, body string) (incident.TimelineEvent, error)
}

// ServiceCatalog resolves service names said in the chat (package service).
type ServiceCatalog interface {
	List(ctx context.Context) ([]service.Service, error)
}

// Config tunes the retrieval: how many runbooks and how similar they must be.
type Config struct {
	TopK          int
	MinSimilarity float64
}

// Source is a runbook the answer is based on. Sources are the runbooks the
// backend retrieved, never something the model claims (UC-10.1).
type Source struct {
	RunbookID   string `json:"runbook_id"`
	Title       string `json:"title"`
	ServiceID   string `json:"service_id"`
	ServiceName string `json:"service_name"`
}

// Action is a tool run while answering, with its outcome.
type Action struct {
	Tool   string `json:"tool"`
	OK     bool   `json:"ok"`
	Result string `json:"result"`
}

// Answer is the copilot's reply to one message.
type Answer struct {
	Reply   string   `json:"reply"`
	Sources []Source `json:"sources"`
	Actions []Action `json:"actions"`
}

// Service runs the conversation.
type Service struct {
	model     Model
	runbooks  Retriever
	incidents Incidents
	notes     Notes
	services  ServiceCatalog
	cfg       Config
}

// NewService builds a Service.
func NewService(model Model, runbooks Retriever, incidents Incidents, notes Notes, services ServiceCatalog, cfg Config) *Service {
	return &Service{model: model, runbooks: runbooks, incidents: incidents, notes: notes, services: services, cfg: cfg}
}

// Ask answers one message. Every tool runs as actor, so the rules and the
// authorship are the same as through the API (BR-14).
func (s *Service) Ask(ctx context.Context, actor user.User, message string) (Answer, error) {
	if incident.Authorize(actor) != nil {
		return Answer{}, ErrForbidden
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return Answer{}, ErrInvalid
	}
	matches, err := s.runbooks.Search(ctx, actor, message, s.cfg.TopK, s.cfg.MinSimilarity)
	if errors.Is(err, runbook.ErrEmbedding) {
		// The embeddings provider is part of the model side: same 503 for the user.
		return Answer{}, fmt.Errorf("%w: %w", ErrModel, err)
	}
	if err != nil {
		return Answer{}, fmt.Errorf("retrieve runbooks: %w", err)
	}

	conv := []Message{
		{Role: RoleSystem, Content: systemPrompt(matches)},
		{Role: RoleUser, Content: message},
	}
	actions := []Action{}
	for range MaxRounds {
		reply, err := s.model.Converse(ctx, conv, tools)
		if err != nil {
			return Answer{}, fmt.Errorf("%w: %w", ErrModel, err)
		}
		if len(reply.ToolCalls) == 0 {
			return s.answer(reply.Content, matches, actions), nil
		}
		conv = append(conv, reply)
		for _, tc := range reply.ToolCalls {
			action := s.run(ctx, actor, tc)
			actions = append(actions, action)
			conv = append(conv, Message{Role: RoleTool, ToolCallID: tc.ID, Name: tc.Name, Content: action.Result})
		}
	}
	return Answer{}, fmt.Errorf("%w: no answer after %d rounds", ErrModel, MaxRounds)
}

// answer applies the RAG guarantees: the sources are the retrieved runbooks,
// and without any of them (and without tools) the reply is the fixed one.
func (s *Service) answer(reply string, matches []runbook.Match, actions []Action) Answer {
	if len(actions) > 0 {
		return Answer{Reply: reply, Sources: []Source{}, Actions: actions}
	}
	if len(matches) == 0 {
		return Answer{Reply: NoInformation, Sources: []Source{}, Actions: actions}
	}
	sources := make([]Source, len(matches))
	for i, m := range matches {
		sources[i] = Source{RunbookID: m.ID, Title: m.Title, ServiceID: m.ServiceID, ServiceName: m.ServiceName}
	}
	return Answer{Reply: reply, Sources: sources, Actions: actions}
}

func systemPrompt(matches []runbook.Match) string {
	var b strings.Builder
	b.WriteString(`Sos el copiloto de Incident Room, una plataforma de gestión de incidentes. Respondé en español, breve.
Reglas:
- Para preguntas sobre cómo proceder, usá SOLO los runbooks de abajo y nombrá el runbook que usás. Si no alcanzan, decí que no encontraste información en los runbooks. Nunca inventes un procedimiento.
- Podés abrir incidentes, agregar notas y listar incidentes abiertos con las herramientas. No podés cerrar, resolver ni transicionar incidentes, ni gestionar runbooks o postmortems: si te lo piden, explicá que no tenés esa herramienta.
- Si una herramienta devuelve un error, explicáselo al usuario; no lo reintentes con otros datos inventados.
- Al abrir un incidente, confirmá su identificador.

Runbooks:
`)
	if len(matches) == 0 {
		b.WriteString("(ninguno relevante)\n")
	}
	for _, m := range matches {
		fmt.Fprintf(&b, "\n[Runbook: %s (servicio %s)]\n%s\n", m.Title, m.ServiceName, m.Content)
	}
	return b.String()
}

// tools are the only actions the copilot can take; closing or transitioning
// an incident is deliberately absent (BR-14, UC-10.6).
var tools = []ToolDef{
	{
		Name:        "abrir_incidente",
		Description: "Declara un incidente nuevo. La severidad es la sugerida por la criticidad del servicio y el impacto.",
		Parameters: object(map[string]any{
			"servicio": prop("string", "Nombre del servicio afectado, por ejemplo payments"),
			"impacto":  enum("Impacto del incidente", "caida_total", "degradacion", "menor"),
			"titulo":   prop("string", "Título breve del incidente"),
		}, "servicio", "impacto", "titulo"),
	},
	{
		Name:        "agregar_nota",
		Description: "Agrega una nota a la timeline de un incidente.",
		Parameters: object(map[string]any{
			"incidente_id": prop("string", "Identificador del incidente"),
			"texto":        prop("string", "Texto de la nota"),
		}, "incidente_id", "texto"),
	},
	{
		Name:        "listar_abiertos",
		Description: "Lista los incidentes activos (no cerrados), con filtros opcionales.",
		Parameters: object(map[string]any{
			"severidad": enum("Filtra por severidad", "SEV1", "SEV2", "SEV3"),
			"servicio":  prop("string", "Filtra por nombre de servicio"),
			"estado":    enum("Filtra por estado", "declarado", "reconocido", "mitigando", "resuelto"),
		}),
	},
}

func object(props map[string]any, required ...string) map[string]any {
	o := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		o["required"] = required
	}
	return o
}

func prop(typ, description string) map[string]any {
	return map[string]any{"type": typ, "description": description}
}

func enum(description string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": description, "enum": values}
}

// run executes one tool call as actor. Failures (rules, bad arguments, unknown
// tools) are returned to the model as text so it can explain them.
func (s *Service) run(ctx context.Context, actor user.User, tc ToolCall) Action {
	result, err := s.dispatch(ctx, actor, tc)
	if err != nil {
		return Action{Tool: tc.Name, OK: false, Result: "error: " + err.Error()}
	}
	return Action{Tool: tc.Name, OK: true, Result: result}
}

func (s *Service) dispatch(ctx context.Context, actor user.User, tc ToolCall) (string, error) {
	var args struct {
		Servicio    string `json:"servicio"`
		Impacto     string `json:"impacto"`
		Titulo      string `json:"titulo"`
		IncidenteID string `json:"incidente_id"`
		Texto       string `json:"texto"`
		Severidad   string `json:"severidad"`
		Estado      string `json:"estado"`
	}
	if tc.Arguments != "" {
		if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
			return "", fmt.Errorf("argumentos inválidos: %w", err)
		}
	}
	switch tc.Name {
	case "abrir_incidente":
		svc, err := s.serviceByName(ctx, args.Servicio)
		if err != nil {
			return "", err
		}
		// No severity: the suggested one applies (BR-01).
		inc, _, err := s.incidents.Declare(ctx, actor, incident.DeclareInput{
			Title: args.Titulo, ServiceID: svc.ID, Impact: incident.Impact(args.Impacto),
		})
		if err != nil {
			return "", err
		}
		return toJSON(map[string]string{"id": inc.ID, "titulo": inc.Title, "severidad": string(inc.Severity), "servicio": svc.Name})
	case "agregar_nota":
		ev, err := s.notes.AddNote(ctx, actor, args.IncidenteID, args.Texto)
		if err != nil {
			return "", err
		}
		return toJSON(map[string]string{"incidente_id": ev.IncidentID, "nota": ev.Body})
	case "listar_abiertos":
		return s.listActive(ctx, actor, args.Severidad, args.Servicio, args.Estado)
	default:
		return "", fmt.Errorf("la herramienta %q no está disponible", tc.Name)
	}
}

func (s *Service) listActive(ctx context.Context, actor user.User, severity, serviceName, state string) (string, error) {
	all, err := s.services.List(ctx)
	if err != nil {
		return "", fmt.Errorf("list services: %w", err)
	}
	names := make(map[string]string, len(all))
	for _, svc := range all {
		names[svc.ID] = svc.Name
	}
	filter := incident.ListFilter{Severity: incident.Severity(severity), State: incident.State(state)}
	if serviceName != "" {
		svc, err := s.serviceByName(ctx, serviceName)
		if err != nil {
			return "", err
		}
		filter.ServiceID = svc.ID
	}
	// The incident service only returns active incidents (BR-15).
	list, err := s.incidents.List(ctx, actor, filter)
	if err != nil {
		return "", err
	}
	type item struct {
		ID        string `json:"id"`
		Titulo    string `json:"titulo"`
		Severidad string `json:"severidad"`
		Estado    string `json:"estado"`
		Servicio  string `json:"servicio"`
	}
	out := make([]item, len(list))
	for i, inc := range list {
		out[i] = item{ID: inc.ID, Titulo: inc.Title, Severidad: string(inc.Severity), Estado: string(inc.State), Servicio: names[inc.ServiceID]}
	}
	return toJSON(out)
}

// serviceByName finds a service by name, ignoring case like its uniqueness.
func (s *Service) serviceByName(ctx context.Context, name string) (service.Service, error) {
	all, err := s.services.List(ctx)
	if err != nil {
		return service.Service{}, fmt.Errorf("list services: %w", err)
	}
	for _, svc := range all {
		if strings.EqualFold(svc.Name, strings.TrimSpace(name)) {
			return svc, nil
		}
	}
	return service.Service{}, fmt.Errorf("el servicio %q no existe", name)
}

func toJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encode tool result: %w", err)
	}
	return string(b), nil
}
