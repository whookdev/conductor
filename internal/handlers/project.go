package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/whookdev/conductor/internal/conductor"
	"github.com/whookdev/conductor/internal/config"
	"github.com/whookdev/conductor/internal/storage"
)

type ProjectHandler struct {
	cfg       *config.Config
	conductor *conductor.Conductor
	storage   *storage.RequestStorage
	logger    *slog.Logger
}

func NewProjectHandler(cfg *config.Config, c *conductor.Conductor, s *storage.RequestStorage, logger *slog.Logger) *ProjectHandler {
	return &ProjectHandler{
		cfg:       cfg,
		conductor: c,
		storage:   s,
		logger:    logger.With("component", "project_handler"),
	}
}

func (h *ProjectHandler) HandleRelayAssignment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectName string `json:"project_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("failed to decode request body", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.ProjectName == "" {
		h.logger.Error("missing project name in request")
		http.Error(w, "project_name is required", http.StatusBadRequest)
		return
	}

	h.logger.Info("assigning relay", "project", req.ProjectName)

	rUrl, err := h.conductor.AssignRelayServer(req.ProjectName)
	if err != nil {
		h.logger.Error("unable to assign relay server",
			"project", req.ProjectName,
			"error", err,
		)
		http.Error(w, "Unable to assign relay server", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"relay_url": "%s"}`, rUrl)
}

func (h *ProjectHandler) HandleProjectRequest(w http.ResponseWriter, r *http.Request) {
	projectName := strings.TrimSuffix(r.Host, "."+h.cfg.BaseDomain)
	h.logger.Info("handling project request",
		"project", projectName,
		"method", r.Method,
		"path", r.URL.Path,
	)

	_, err := h.storage.StoreRequest(r, projectName)
	if err != nil {
		h.logger.Error("failed to store request", "project", projectName, "error", err)
	}

	relayURL, err := h.conductor.GetProjectRelayServer(projectName)
	if err != nil {
		h.logger.Error("unable to get relay server",
			"project", projectName,
			"error", err,
		)
		//TODO: This is a difficult one, if we return a non-200 most services will
		//attempt to retry the webhook. If we return a 200 and have stored it we
		//can replay the webhook and the caller only sees a success. Need to figure
		//out what the ideal flow is here - is it better to 'lie' to the caller and
		//claim everything is OK, or do we inform it that the downstream service
		//was unreachable? If we 'lie', we can then store these messages in a queue
		//for when a relay server is accessible again? We should also add some
		//fault tolerance that tries to reassign a project to a relay server
		http.Error(w, "Unable to process request", http.StatusInternalServerError)
		return
	}

	h.logger.Info("relay URL found", "relay_url", relayURL)

	ForwardRequest(w, r, relayURL, h.cfg.BaseDomain, h.logger)
}
