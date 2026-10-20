// Command api is the entry point of the Incident Room backend.
//
// Configuration comes from the environment: DATABASE_URL (required), PORT
// (default 8080), ADMIN_EMAIL and ADMIN_PASSWORD (optional; when both are set
// the first admin is created if none exists, BR-19), SLA_SEV1, SLA_SEV2 and
// SLA_SEV3 (acknowledgement deadlines as Go durations, defaults 5m, 15m and 60m,
// BR-03) and ESCALATION_INTERVAL (how often overdue incidents are escalated,
// default 10s, BR-04). STATIC_DIR (optional) is a compiled frontend directory
// that holds index.html; when set, the server also serves it outside /api.
// Pending SQL migrations are applied at startup.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"incident-room-backend/internal/auth"
	"incident-room-backend/internal/copilot"
	"incident-room-backend/internal/httpapi"
	"incident-room-backend/internal/incident"
	"incident-room-backend/internal/llm"
	"incident-room-backend/internal/metrics"
	"incident-room-backend/internal/postmortem"
	"incident-room-backend/internal/runbook"
	"incident-room-backend/internal/service"
	"incident-room-backend/internal/timeline"
	"incident-room-backend/internal/user"
	"incident-room-backend/migrations"
)

const (
	sessionTTL                = 24 * time.Hour
	defaultEscalationInterval = 10 * time.Second
	// Embeddings always come from Ollama; 768 dimensions match the runbooks table (UC-07).
	defaultOllamaURL      = "http://localhost:11434"
	defaultEmbeddingModel = "nomic-embed-text"
	// Chat (UC-08, UC-10): Ollama by default; Groq through its OpenAI-compatible API.
	defaultOllamaChatModel = "llama3.1"
	defaultGroqURL         = "https://api.groq.com/openai/v1"
	defaultGroqChatModel   = "llama-3.1-8b-instant"
	// Copilot RAG (UC-10): runbooks used per question and their minimum similarity.
	ragTopK              = 3
	defaultMinSimilarity = 0.5
	shutdownTimeout      = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir != "" {
		fi, err := os.Stat(filepath.Join(staticDir, "index.html"))
		if err != nil {
			return fmt.Errorf("STATIC_DIR %q must contain index.html: %w", staticDir, err)
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("STATIC_DIR %q: index.html must be a regular file", staticDir)
		}
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	if err := migrations.Apply(ctx, pool); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	userRepo := user.NewPostgresRepository(pool)
	userSvc := user.NewService(userRepo)
	authSvc := auth.NewService(userRepo, auth.NewPostgresSessionRepository(pool), sessionTTL, time.Now)

	sla, err := slaFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	escalationInterval, err := escalationIntervalFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	serviceMgr := service.NewManager(service.NewPostgresRepository(pool), userRepo, time.Now)
	incidentRepo := incident.NewPostgresRepository(pool)
	incidentSvc := incident.NewService(incidentRepo, time.Now, sla)
	timelineSvc := timeline.NewService(incidentRepo, time.Now)
	metricsSvc := metrics.NewService(metrics.NewPostgresRepository(pool), time.Now, sla)
	embedder, err := llm.NewOllamaEmbedder(envOr(os.Getenv, "OLLAMA_URL", defaultOllamaURL), envOr(os.Getenv, "EMBEDDING_MODEL", defaultEmbeddingModel))
	if err != nil {
		return err
	}
	runbookSvc := runbook.NewService(runbook.NewPostgresRepository(pool), embedder, time.Now)
	chatCfg, err := chatConfigFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	chat, err := chatCfg.build()
	if err != nil {
		return err
	}
	postmortemSvc := postmortem.NewService(incidentRepo, postmortem.NewPostgresRepository(pool), chat, time.Now)
	minSimilarity, err := minSimilarityFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	copilotSvc := copilot.NewService(chat, runbookSvc, incidentSvc, timelineSvc, serviceMgr,
		copilot.Config{TopK: ragTopK, MinSimilarity: minSimilarity})

	if email, password := os.Getenv("ADMIN_EMAIL"), os.Getenv("ADMIN_PASSWORD"); email != "" && password != "" {
		if err := userSvc.EnsureAdmin(ctx, email, password); err != nil {
			if errors.Is(err, user.ErrAdminEmailTaken) {
				return fmt.Errorf("ADMIN_EMAIL %q already belongs to a non-admin user and there is no admin: "+
					"existing users are not promoted automatically; use a different ADMIN_EMAIL or remove that user: %w", email, err)
			}
			return fmt.Errorf("ensure admin: %w", err)
		}
	}

	// Deferred after pool.Close, so it runs first: the ticker goroutine must
	// finish before the pool is closed.
	escalationDone := make(chan struct{})
	defer func() {
		stop()
		<-escalationDone
	}()
	go func() {
		defer close(escalationDone)
		runEscalation(ctx, incidentSvc, escalationInterval, logger)
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	handler := httpapi.New(userSvc, authSvc, serviceMgr, incidentSvc, timelineSvc, metricsSvc, runbookSvc, postmortemSvc, copilotSvc, logger)
	if staticDir != "" {
		handler = httpapi.Static(staticDir, handler)
		logger.Info("serving API and frontend", "mode", "production", "static_dir", staticDir)
	} else {
		logger.Info("serving API only", "mode", "development")
	}
	log.Printf("listening on :%s", port)
	srv := &http.Server{Addr: ":" + port, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}
	log.Printf("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}
	return nil
}

// overdueEscalator is the part of the incident service the ticker needs.
type overdueEscalator interface {
	EscalateOverdue(ctx context.Context) (int, error)
}

// runEscalation evaluates overdue incidents every interval until ctx is done.
// A failed evaluation is logged and never stops the server (BR-04). There is
// no evaluation before the first tick.
func runEscalation(ctx context.Context, svc overdueEscalator, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := svc.EscalateOverdue(ctx); err != nil {
				logger.Error("escalate overdue incidents", "err", err, "escalated", n)
			}
		}
	}
}

// chatConfig selects the chat provider (Ollama or Groq) without touching code.
type chatConfig struct {
	provider, baseURL, apiKey, model string
}

// chatConfigFromEnv reads LLM_PROVIDER (ollama by default, or groq) and
// LLM_MODEL. Both are reached through the OpenAI-compatible API: Ollama under
// OLLAMA_URL/v1 (it needs no key), Groq at LLM_BASE_URL with GROQ_API_KEY. A
// missing key or an unknown provider stops the start.
func chatConfigFromEnv(getenv func(string) string) (chatConfig, error) {
	switch provider := envOr(getenv, "LLM_PROVIDER", "ollama"); provider {
	case "ollama":
		return chatConfig{
			provider: provider,
			baseURL:  strings.TrimSuffix(envOr(getenv, "OLLAMA_URL", defaultOllamaURL), "/") + "/v1",
			// Ollama ignores the key, but the OpenAI client refuses an empty one.
			apiKey: "ollama",
			model:  envOr(getenv, "LLM_MODEL", defaultOllamaChatModel),
		}, nil
	case "groq":
		key := getenv("GROQ_API_KEY")
		if key == "" {
			return chatConfig{}, errors.New("LLM_PROVIDER=groq requires GROQ_API_KEY")
		}
		return chatConfig{
			provider: provider,
			baseURL:  envOr(getenv, "LLM_BASE_URL", defaultGroqURL),
			apiKey:   key,
			model:    envOr(getenv, "LLM_MODEL", defaultGroqChatModel),
		}, nil
	default:
		return chatConfig{}, fmt.Errorf("LLM_PROVIDER must be ollama or groq, got %q", provider)
	}
}

// String hides the API key, so logging the configuration never leaks it.
func (c chatConfig) String() string {
	key := ""
	if c.apiKey != "" {
		key = "***"
	}
	return fmt.Sprintf("{provider:%s baseURL:%s apiKey:%s model:%s}", c.provider, c.baseURL, key, c.model)
}

func (c chatConfig) build() (*llm.Chat, error) {
	return llm.NewChat(c.baseURL, c.apiKey, c.model)
}

// minSimilarityFromEnv reads RAG_MIN_SIMILARITY, the cosine similarity a
// runbook needs to be used by the copilot. It depends on the embeddings model,
// so it is tunable; the default suits nomic-embed-text.
func minSimilarityFromEnv(getenv func(string) string) (float64, error) {
	v := getenv("RAG_MIN_SIMILARITY")
	if v == "" {
		return defaultMinSimilarity, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 0 || f > 1 {
		return 0, fmt.Errorf("RAG_MIN_SIMILARITY must be a number between 0 and 1, got %q", v)
	}
	return f, nil
}

// envOr returns the variable name, or fallback when it is unset or empty.
func envOr(getenv func(string) string, name, fallback string) string {
	if v := getenv(name); v != "" {
		return v
	}
	return fallback
}

// slaFromEnv reads SLA_SEV1, SLA_SEV2 and SLA_SEV3 over the defaults (BR-03).
// An invalid or non-positive value is an error, so a typo fails at startup.
func slaFromEnv(getenv func(string) string) (incident.SLA, error) {
	sla := incident.DefaultSLA()
	for _, f := range []struct {
		name string
		dst  *time.Duration
	}{{"SLA_SEV1", &sla.SEV1}, {"SLA_SEV2", &sla.SEV2}, {"SLA_SEV3", &sla.SEV3}} {
		v := getenv(f.name)
		if v == "" {
			continue
		}
		d, err := parsePositiveDuration(f.name, v)
		if err != nil {
			return incident.SLA{}, err
		}
		*f.dst = d
	}
	return sla, nil
}

// escalationIntervalFromEnv reads ESCALATION_INTERVAL.
func escalationIntervalFromEnv(getenv func(string) string) (time.Duration, error) {
	v := getenv("ESCALATION_INTERVAL")
	if v == "" {
		return defaultEscalationInterval, nil
	}
	return parsePositiveDuration("ESCALATION_INTERVAL", v)
}

func parsePositiveDuration(name, v string) (time.Duration, error) {
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s %q is not a valid duration: %w", name, v, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %q", name, v)
	}
	return d, nil
}
