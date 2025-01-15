package models

import "time"

type StoredRequest struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Headers     map[string]string `json:"headers"`
	Body        []byte            `json:"body"`
	ProjectName string            `json:"project_name"`
	ReceivedAt  time.Time         `json:"received_at"`
}
