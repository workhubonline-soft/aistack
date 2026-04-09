package ollama

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "http://localhost:11434"

// Client communicates with the Ollama REST API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates an Ollama API client. Pass "" to use http://localhost:11434.
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// PullProgress is emitted during model download.
type PullProgress struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
}

// Pull downloads a model, calling progressFn for each status update.
func (c *Client) Pull(model string, progressFn func(PullProgress)) error {
	body, err := json.Marshal(map[string]string{"name": model})
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/api/pull", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("connecting to Ollama at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body) //nolint:errcheck // best-effort error body
		return fmt.Errorf("ollama pull failed (HTTP %d): %s", resp.StatusCode, string(msg))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var p PullProgress
		if err := json.Unmarshal(scanner.Bytes(), &p); err != nil {
			continue
		}
		if progressFn != nil {
			progressFn(p)
		}
	}
	return scanner.Err()
}

// Model represents a locally available model.
type Model struct {
	Name       string    `json:"name"`
	ModifiedAt time.Time `json:"modified_at"`
	Size       int64     `json:"size"`
	Digest     string    `json:"digest"`
}

// List returns all locally available models.
func (c *Client) List() ([]Model, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("connecting to Ollama at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body) //nolint:errcheck // best-effort error body
		return nil, fmt.Errorf("ollama list failed (HTTP %d): %s", resp.StatusCode, string(msg))
	}

	var result struct {
		Models []Model `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return result.Models, nil
}

// GenerateRequest is the payload for /api/generate.
type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// GenerateResponse is the non-streaming response from /api/generate.
type GenerateResponse struct {
	Response           string `json:"response"`
	TotalDuration      int64  `json:"total_duration"`
	LoadDuration       int64  `json:"load_duration"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalCount          int    `json:"eval_count"`
	EvalDuration       int64  `json:"eval_duration"`
}

// Generate sends a prompt and returns the full response (non-streaming).
func (c *Client) Generate(model, prompt string) (*GenerateResponse, error) {
	body, err := json.Marshal(GenerateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	// Use a separate client with a generous timeout for generation.
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Post(c.baseURL+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("connecting to Ollama at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body) //nolint:errcheck // best-effort error body
		return nil, fmt.Errorf("ollama generate failed (HTTP %d): %s", resp.StatusCode, string(msg))
	}

	var result GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &result, nil
}

// Ping checks if Ollama is reachable.
func (c *Client) Ping() error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(c.baseURL + "/api/tags")
	if err != nil {
		return fmt.Errorf("Ollama is not reachable at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama returned HTTP %d", resp.StatusCode)
	}
	return nil
}
