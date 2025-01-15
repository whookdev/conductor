package storage

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/whookdev/conductor/internal/models"
)

type RequestStorage struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *RequestStorage {
	return &RequestStorage{
		logger: logger.With("component", "request_storage"),
	}
}

func (s *RequestStorage) StoreRequest(r *http.Request, projectName string) (*models.StoredRequest, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	headers := make(map[string]string)
	for name, values := range r.Header {
		headers[name] = values[0]
	}

	storedReq := &models.StoredRequest{
		Method:      r.Method,
		Path:        r.URL.Path,
		Headers:     headers,
		Body:        bodyBytes,
		ProjectName: projectName,
		ReceivedAt:  time.Now(),
	}

	s.logger.Info("stored request",
		"project", projectName,
		"method", storedReq.Method,
		"path", storedReq.Path,
		"received_at", storedReq.ReceivedAt,
	)

	return storedReq, nil
}
