package jira

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client represents a Jira API client
type Client struct {
	baseURL    string
	email      string
	apiToken   string
	httpClient *http.Client
}

// NewClient creates a new Jira API client
func NewClient(baseURL, email, apiToken string) *Client {
	// Remove trailing slash from baseURL if present
	baseURL = strings.TrimRight(baseURL, "/")

	return &Client{
		baseURL:    baseURL,
		email:      email,
		apiToken:   apiToken,
		httpClient: &http.Client{},
	}
}

// doRequest performs an HTTP request with authentication and error handling
func (c *Client) doRequest(method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader

	// Marshal request body if provided
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	// Construct full URL
	url := c.baseURL + path

	// Create HTTP request
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set Basic Auth header
	auth := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.apiToken))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Handle error status codes
	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && len(errResp.ErrorMessages) > 0 {
			return fmt.Errorf("jira api error (status %d): %s", resp.StatusCode, strings.Join(errResp.ErrorMessages, ", "))
		}
		return fmt.Errorf("jira api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Unmarshal response if result pointer is provided
	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}
