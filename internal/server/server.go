package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/whookdev/conductor/internal/conductor"
	"github.com/whookdev/conductor/internal/config"
	"github.com/whookdev/conductor/internal/handlers"
	"github.com/whookdev/conductor/internal/storage"
)

type Server struct {
	cfg            *config.Config
	conductor      *conductor.Conductor
	server         *http.Server
	logger         *slog.Logger
	projectHandler *handlers.ProjectHandler
}

func New(cfg *config.Config, tc *conductor.Conductor, logger *slog.Logger) (*Server, error) {
	requestStorage := storage.New(logger)
	projectHandler := handlers.NewProjectHandler(cfg, tc, requestStorage, logger)

	logger = logger.With("component", "server")

	s := &Server{
		cfg:            cfg,
		conductor:      tc,
		logger:         logger,
		projectHandler: projectHandler,
	}

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      s.routes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	go func() {
		s.logger.Info("starting server", "address", s.server.Addr)
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			s.logger.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	return s.Shutdown()
}

func (s *Server) Shutdown() error {
	s.logger.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return s.server.Shutdown(ctx)
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleRequest)

	return mux
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if !strings.HasSuffix(host, s.cfg.BaseDomain) {
		s.logger.Warn("request with invalid domain",
			"host", host,
			"expected_domain", s.cfg.BaseDomain,
			"remote_addr", r.RemoteAddr)
		http.Error(w, "Invalid domain", http.StatusBadRequest)
		return
	}

	if strings.HasPrefix(host, "api.") {
		if r.Method == http.MethodPost && r.URL.Path == "/relay" {
			s.projectHandler.HandleRelayAssignment(w, r)
			return
		}
		http.Error(w, "Not found", http.StatusNotFound)
		return
	} else {
		s.projectHandler.HandleProjectRequest(w, r)
	}
}
