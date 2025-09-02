package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ConfigClient handles configuration API calls
type ConfigClient struct {
	baseURL     string
	tokenGetter func() string
	client      *http.Client
}

// NewConfigClient creates a new config client
func NewConfigClient(baseURL string, tokenGetter func() string) *ConfigClient {
	return &ConfigClient{
		baseURL:     baseURL,
		tokenGetter: tokenGetter,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetEDLConfig fetches the EDL configuration
func (c *ConfigClient) GetEDLConfig(ctx context.Context) (*EDLConfig, error) {
	// Use the config URL directly as provided by bootstrap response
	// The URL already contains the complete path
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL, nil)
	if err != nil {
		return nil, err
	}

	// Add authorization header
	token := c.tokenGetter()
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 410 {
		return nil, &APIError{
			StatusCode: 410,
			Message:    "deployment permanently deleted",
		}
	}

	if resp.StatusCode == 403 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, &APIError{
			StatusCode: 403,
			Message:    fmt.Sprintf("deployment temporarily disabled: %s", string(bodyBytes)),
		}
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("config fetch failed with status %d: %s", resp.StatusCode, string(bodyBytes)),
		}
	}

	var config EDLConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
