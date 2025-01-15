package server

import (
	"context"
	"encoding/json"
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
		s.logger.Warn("request with invalid domain",
			"host", host,
			"expected_domain", s.cfg.BaseDomain,
			"remote_addr", r.RemoteAddr)
		http.Error(w, "Invalid domain", http.StatusBadRequest)
		return
	}

	if strings.HasPrefix(host, "api.") {
		if r.Method == http.MethodPost && r.URL.Path == "/tunnel" {
			s.handleTunnelAssignment(w, r)
			return
		}
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	s.handleProjectRequest(w, r)
}

func (s *Server) handleTunnelAssignment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectName string `json:"project_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("failed to decode request body", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
	}
	defer r.Body.Close()

	if req.ProjectName == "" {
		s.logger.Error("missing project name in request")
		http.Error(w, "project_name is required", http.StatusBadRequest)
	}

	s.logger.Info("assigning tunnel",
		"project", req.ProjectName,
	)

	tUrl, err := s.conductor.AssignTunnelServer(req.ProjectName)
	if err != nil {
		s.logger.Error("unable to assign tunnel server",
			"project", req.ProjectName,
			"error", err)
		http.Error(w, "Unable to assign tunnel server", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"tunnel_url":"%s"}`, tUrl)
}

func (s *Server) handleProjectRequest(w http.ResponseWriter, r *http.Request) {
	projectName := strings.TrimSuffix(r.Host, "."+s.cfg.BaseDomain)
	s.logger.Info("handling project request",
		"project", projectName,
		"method", r.Method,
		"path", r.URL.Path,
	)

	relayURL, err := s.conductor.GetProjectRelayServer(projectName)
	if err != nil {
		s.logger.Error("unable to get relay server",
			"project", projectName,
			"error", err)
		http.Error(w, "Unable to process request", http.StatusInternalServerError)
		return
	}
	s.logger.Info("Relay url found", "relay_url", relayURL)

	http.Error(w, "Forwarding not implemented", http.StatusNotImplemented)
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
