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
)

type Server struct {
	cfg       *config.Config
	conductor *conductor.Conductor
	server    *http.Server
	logger    *slog.Logger
}

func New(cfg *config.Config, tc *conductor.Conductor, logger *slog.Logger) (*Server, error) {
	logger = logger.With("component", "server")

	s := &Server{
		cfg:       cfg,
		conductor: tc,
		logger:    logger,
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
		s.logger.Error("invalid domain")
		http.Error(w, "Invalid domain", http.StatusBadRequest)
	}

	projectName := strings.TrimSuffix(host, "."+s.cfg.BaseDomain)
	s.logger.Info("received request",
		"project", projectName,
		"method", r.Method,
		"path", r.URL.Path,
	)

	// Let's assign a server
	tUrl, err := s.conductor.AssignTunnelServer(projectName)
	if err != nil {
		s.logger.Error("unable to assign tunnel server for", "projectName", fmt.Errorf("%w", err))
		http.Error(w, "Unable to assign tunnel server", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"tunnelUrl":"%s"}`, tUrl)

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
