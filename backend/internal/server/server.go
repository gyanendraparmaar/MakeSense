package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/gyanendraparmaar/makesense/backend/internal/config"
	"github.com/gyanendraparmaar/makesense/backend/internal/llm"
	"github.com/gyanendraparmaar/makesense/backend/internal/storage"
)

// Server bundles the deps handlers need.
type Server struct {
	cfg      config.Config
	store    *storage.Store
	pipeline *llm.Pipeline
}

func New(cfg config.Config, store *storage.Store, pipeline *llm.Pipeline) *Server {
	return &Server{cfg: cfg, store: store, pipeline: pipeline}
}

// Router returns the HTTP router with middleware and routes wired up.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(90 * time.Second))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{s.cfg.AllowedOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/health", s.handleHealth)

	r.Route("/api", func(r chi.Router) {
		r.Post("/analyze", s.handleAnalyze)         // JSON response
		r.Post("/analyze/stream", s.handleAnalyzeStream) // SSE
		r.Route("/notes", func(r chi.Router) {
			r.Get("/", s.handleListNotes)
			r.Post("/", s.handleCreateNote)
			r.Get("/{id}", s.handleGetNote)
			r.Put("/{id}", s.handleUpdateNote)
			r.Delete("/{id}", s.handleDeleteNote)
		})
	})

	return r
}
