package nri

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// nriWriter handles writing NRI payloads to various outputs
type nriWriter interface {
	write(payload interface{}) error
	close() error
}

// fileWriter writes NRI payloads to files
type fileWriter struct {
	path string
	mu   sync.Mutex
}

// stdoutWriter writes NRI payloads to stdout
type stdoutWriter struct {
	encoder *json.Encoder
	mu      sync.Mutex
}

// httpWriter writes NRI payloads to HTTP endpoints
type httpWriter struct {
	endpoint string
	client   *http.Client
	mu       sync.Mutex
}

// newNRIWriter creates a new NRI writer based on the output mode
func newNRIWriter(cfg *Config) (nriWriter, error) {
	switch cfg.OutputMode {
	case "file":
		return &fileWriter{path: cfg.OutputPath}, nil
	case "stdout":
		return &stdoutWriter{encoder: json.NewEncoder(os.Stdout)}, nil
	case "http":
		return &httpWriter{
			endpoint: cfg.HTTPEndpoint,
			client: &http.Client{
				Timeout: cfg.Timeout,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported output mode: %s", cfg.OutputMode)
	}
}

// write implements nriWriter for fileWriter
func (w *fileWriter) write(payload interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(w.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Open file in append mode
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Write JSON payload
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(payload); err != nil {
		return fmt.Errorf("failed to encode payload: %w", err)
	}

	return nil
}

// close implements nriWriter for fileWriter
func (w *fileWriter) close() error {
	// Nothing to close for file writer
	return nil
}

// write implements nriWriter for stdoutWriter
func (w *stdoutWriter) write(payload interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.encoder.Encode(payload); err != nil {
		return fmt.Errorf("failed to encode payload: %w", err)
	}

	return nil
}

// close implements nriWriter for stdoutWriter
func (w *stdoutWriter) close() error {
	// Nothing to close for stdout writer
	return nil
}

// write implements nriWriter for httpWriter
func (w *httpWriter) write(payload interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Marshal payload to JSON
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", w.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// close implements nriWriter for httpWriter
func (w *httpWriter) close() error {
	// Nothing to close for HTTP writer
	return nil
}