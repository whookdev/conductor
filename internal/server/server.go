package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/whookdev/conductor/internal/config"
)

type Server struct {
	cfg *config.Config
	// tunnelManager *tunnel.Manager
	server *http.Server
	logger *slog.Logger
}

func New(cfg *config.Config) (*Server, error) {
	logger := slog.With("component", "server")

	// tunnel manager

	s := &Server{
		cfg:    cfg,
		logger: logger,
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

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleRequest)

	return mux
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if !strings.HasSuffix(host, s.cfg.BaseDomain) {
		http.Error(w, "Invalid domain", http.StatusBadRequest)
	}

	projectName := strings.TrimSuffix(host, "."+s.cfg.BaseDomain)
	s.logger.Info("received request",
		"project", projectName,
		"method", r.Method,
		"path", r.URL.Path,
	)
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
	s.logger.Info("shuttong down server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return s.server.Shutdown(ctx)
}
