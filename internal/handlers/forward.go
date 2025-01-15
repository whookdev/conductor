package handlers

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func ForwardRequest(w http.ResponseWriter, r *http.Request, relayURL string, baseDomain string, logger *slog.Logger) {
	logger = logger.With("component", "forward_request")

	if !strings.HasPrefix(relayURL, "http://") && !strings.HasPrefix(relayURL, "https://") {
		relayURL = "http://" + relayURL
	}

	targetURL := fmt.Sprintf("%s%s", relayURL, r.URL.RequestURI())

	proxyReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		logger.Error("failed to create proxy request", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	proxyReq.Header.Set("X-Forwarded-Host", r.Host)
	proxyReq.Header.Set("X-Original-URL", r.URL.String())
	proxyReq.Header.Set("X-Project-Name", strings.TrimSuffix(r.Host, "."+baseDomain)) // TODO: Just send the bloody project name in
	proxyReq.Header.Set("X-Received-At", fmt.Sprintf("%d", time.Now().Unix()))

	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		logger.Error("failed to forward request", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		logger.Error("failed to copy response body", "error", err)
		return
	}
}
