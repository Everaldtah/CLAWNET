package openclaw

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Everaldtah/CLAWNET/internal/config"
)

// Client is an OpenClaw API client
type Client struct {
	endpoint   string
	httpClient *http.Client
	config     config.OpenClawConfig
}

// Request represents an OpenClaw completion request
type Request struct {
	Prompt      string  `json:"prompt"`
	Context     string  `json:"context,omitempty"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
	Model       string  `json:"model"`
	Stream      bool    `json:"stream,omitempty"`
}

// Response represents an OpenClaw completion response
type Response struct {
	Text         string `json:"text"`
	TokensUsed   int    `json:"tokens_used"`
	FinishReason string `json:"finish_reason"`
	Stream       bool   `json:"stream,omitempty"`
}

// NewClient creates a new OpenClaw client
func NewClient(cfg config.OpenClawConfig) (*Client, error) {
	return &Client{
		endpoint: cfg.APIEndpoint,
		httpClient: &http.Client{
			Timeout: cfg.APITimeout,
		},
		config: cfg,
	}, nil
}

// Complete sends a completion request
func (c *Client) Complete(ctx context.Context, req *Request) (*Response, error) {
	if req.Stream {
		return nil, fmt.Errorf("streaming not supported in Complete, use CompleteStream")
	}

	payload := map[string]interface{}{
		"prompt":      req.Prompt,
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
		"model":       req.Model,
	}

	if req.Context != "" {
		payload["context"] = req.Context
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/v1/complete", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Text         string `json:"text"`
		TokensUsed   int    `json:"tokens_used"`
		FinishReason string `json:"finish_reason"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &Response{
		Text:         result.Text,
		TokensUsed:   result.TokensUsed,
		FinishReason: result.FinishReason,
	}, nil
}

// CompleteStream sends a streaming completion request
func (c *Client) CompleteStream(ctx context.Context, req *Request) (<-chan *Response, error) {
	payload := map[string]interface{}{
		"prompt":      req.Prompt,
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
		"model":       req.Model,
		"stream":      true,
	}

	if req.Context != "" {
		payload["context"] = req.Context
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/v1/complete", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	responseChan := make(chan *Response, 100)

	go func() {
		defer close(responseChan)
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					responseChan <- &Response{
						Text:         "",
						FinishReason: "error",
						Stream:       true,
					}
				}
				return
			}

			line = strings.TrimSpace(line)
			if len(line) == 0 {
				continue
			}

			// Parse SSE format
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				
				if string(data) == "[DONE]" {
					responseChan <- &Response{
						Text:         "",
						FinishReason: "stop",
						Stream:       true,
					}
					return
				}

				var chunk struct {
					Text       string `json:"text"`
					FinishReason string `json:"finish_reason,omitempty"`
				}

				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					continue
				}

				responseChan <- &Response{
					Text:         chunk.Text,
					FinishReason: chunk.FinishReason,
					Stream:       true,
				}

				if chunk.FinishReason != "" {
					return
				}
			}
		}
	}()

	return responseChan, nil
}

// Health checks the OpenClaw API health
func (c *Client) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.endpoint+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %s", resp.Status)
	}

	return nil
}

// GetModels gets available models
func (c *Client) GetModels(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.endpoint+"/v1/models", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get models: %s", resp.Status)
	}

	var result struct {
		Models []string `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Models, nil
}
