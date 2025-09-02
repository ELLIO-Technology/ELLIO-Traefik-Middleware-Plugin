package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// BootstrapRequest represents the bootstrap API request
type BootstrapRequest struct {
	BootstrapToken   string   `json:"bootstrap_token"`
	ComponentType    string   `json:"component_type"`
	ComponentVersion string   `json:"component_version"`
	MachineID        string   `json:"machine_id"`
	Scopes           []string `json:"scopes,omitempty"`
}

// BootstrapClaims for parsing JWT to get issuer
type BootstrapClaims struct {
	ComponentType string `json:"component_type"`
	DeploymentID  string `json:"deployment_id"`
	jwt.RegisteredClaims
}

// BootstrapClient handles bootstrap API calls
type BootstrapClient struct {
	client *http.Client
}

// NewBootstrapClient creates a new bootstrap client
func NewBootstrapClient() *BootstrapClient {
	return &BootstrapClient{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Bootstrap performs the bootstrap operation with issuer-based URL
// IMPORTANT: We use manual JWT parsing here due to Yaegi's incompatibility with jwt/v5
// struct tags. See: https://github.com/traefik/yaegi/discussions/1548
func (c *BootstrapClient) Bootstrap(ctx context.Context, token string, machineID string) (*BootstrapResponse, error) {
	// Manual JWT parsing to work around Yaegi limitation with jwt/v5
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Decode the payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	// Parse JSON manually
	var rawClaims map[string]interface{}
	if err := json.Unmarshal(payload, &rawClaims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	// Extract issuer and component_type
	issuer, ok := rawClaims["iss"].(string)
	if !ok || issuer == "" {
		return nil, fmt.Errorf("bootstrap token missing issuer")
	}

	componentType, _ := rawClaims["component_type"].(string)

	// Construct bootstrap URL from issuer
	bootstrapURL := strings.TrimSuffix(issuer, "/") + "/api/v1/edl/bootstrap"

	// Create bootstrap request
	req := BootstrapRequest{
		BootstrapToken:   token,
		ComponentType:    componentType,
		ComponentVersion: "1.0.0",
		MachineID:        machineID,
		Scopes:           []string{"edl_config", "edl_logs"},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", bootstrapURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
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
			Message:    fmt.Sprintf("bootstrap failed with status %d: %s", resp.StatusCode, string(bodyBytes)),
		}
	}

	var result BootstrapResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
