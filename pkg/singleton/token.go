package singleton

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/api"
	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/logger"
	"github.com/golang-jwt/jwt/v5"
)

// TokenManager manages JWT tokens and refreshing
type TokenManager struct {
	bootstrapClient *api.BootstrapClient
	bootstrapToken  string
	machineID       string

	mu                sync.RWMutex
	currentToken      string
	tokenExpiry       time.Time
	configURL         string
	logsURL           string
	deploymentDeleted bool

	stopCh chan struct{}
}

// BootstrapClaims represents the JWT claims in the bootstrap token
// Note: We manually parse these due to Yaegi's issues with jwt/v5 struct tags
// See: https://github.com/traefik/yaegi/discussions/1548
type BootstrapClaims struct {
	ComponentType string `json:"component_type"`
	DeploymentID  string `json:"deployment_id"`
	Issuer        string `json:"iss"`
	jwt.RegisteredClaims
}

// NewTokenManager creates a new token manager
func NewTokenManager(bootstrapToken string, machineID string) *TokenManager {
	return &TokenManager{
		bootstrapClient: api.NewBootstrapClient(),
		bootstrapToken:  bootstrapToken,
		machineID:       machineID,
		stopCh:          make(chan struct{}),
	}
}

// ParseBootstrapToken parses and validates the bootstrap token
// IMPORTANT: We use manual JWT parsing instead of jwt/v5's ParseUnverified because
// Yaegi (Traefik's Go interpreter) has issues with struct tags in jwt/v5, causing
// claims to be returned as empty. This is a known Yaegi limitation.
// See: https://github.com/traefik/yaegi/discussions/1548
func (tm *TokenManager) ParseBootstrapToken() (*BootstrapClaims, error) {
	// Manual JWT parsing to work around Yaegi limitation
	parts := strings.Split(tm.bootstrapToken, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid JWT format")
	}

	// Decode the payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("failed to decode JWT payload: " + err.Error())
	}

	// Parse JSON manually
	var rawClaims map[string]interface{}
	if err := json.Unmarshal(payload, &rawClaims); err != nil {
		return nil, errors.New("failed to parse JWT claims: " + err.Error())
	}

	// Extract fields manually
	claims := &BootstrapClaims{}

	if ct, ok := rawClaims["component_type"].(string); ok {
		claims.ComponentType = ct
	}

	if did, ok := rawClaims["deployment_id"].(string); ok {
		claims.DeploymentID = did
	}

	if iss, ok := rawClaims["iss"].(string); ok {
		claims.Issuer = iss
	}

	return claims, nil
}

// Initialize performs initial bootstrap
func (tm *TokenManager) Initialize(ctx context.Context) error {
	resp, err := tm.bootstrapClient.Bootstrap(ctx, tm.bootstrapToken, tm.machineID)
	if err != nil {
		if api.IsPermanentError(err) {
			tm.mu.Lock()
			tm.deploymentDeleted = true
			tm.mu.Unlock()
			logger.Info("Deployment permanently deleted (410), switching to allow-all mode")
		}
		return err
	}

	tm.mu.Lock()
	tm.currentToken = resp.AccessToken
	tm.tokenExpiry = time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
	tm.configURL = resp.ConfigURL
	tm.logsURL = resp.LogsURL
	tm.mu.Unlock()

	logger.Debugf("Bootstrap successful, token expires in %d seconds", resp.ExpiresIn)
	logger.Debugf("Config URL from bootstrap: %s", resp.ConfigURL)
	if resp.LogsURL != "" {
		logger.Debugf("Logs URL from bootstrap: %s", resp.LogsURL)
	}
	return nil
}

// StartRefreshLoop starts the background token refresh loop
func (tm *TokenManager) StartRefreshLoop(ctx context.Context) {
	// Don't start if deployment is deleted
	tm.mu.RLock()
	if tm.deploymentDeleted {
		tm.mu.RUnlock()
		return
	}
	tm.mu.RUnlock()

	refreshTimer := time.NewTimer(tm.calculateRefreshInterval())
	defer refreshTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tm.stopCh:
			return
		case <-refreshTimer.C:
			tm.mu.RLock()
			deleted := tm.deploymentDeleted
			tm.mu.RUnlock()

			if deleted {
				logger.Info("Stopping token refresh - deployment deleted")
				return
			}

			if err := tm.refresh(ctx); err != nil {
				logger.Warnf("Token refresh failed: %v", err)
				// Retry after 30 seconds
				refreshTimer.Reset(30 * time.Second)
			} else {
				refreshTimer.Reset(tm.calculateRefreshInterval())
			}
		}
	}
}

// calculateRefreshInterval calculates when to refresh (80% of token lifetime)
func (tm *TokenManager) calculateRefreshInterval() time.Duration {
	tm.mu.RLock()
	expiry := tm.tokenExpiry
	tm.mu.RUnlock()

	timeUntilExpiry := time.Until(expiry)
	refreshAt := time.Duration(float64(timeUntilExpiry) * 0.8)

	// Minimum 30 seconds
	if refreshAt < 30*time.Second {
		refreshAt = 30 * time.Second
	}

	return refreshAt
}

// refresh refreshes the token
func (tm *TokenManager) refresh(ctx context.Context) error {
	resp, err := tm.bootstrapClient.Bootstrap(ctx, tm.bootstrapToken, tm.machineID)
	if err != nil {
		if api.IsPermanentError(err) {
			tm.mu.Lock()
			tm.deploymentDeleted = true
			tm.mu.Unlock()
			logger.Info("Deployment deleted during refresh (410)")
		}
		return err
	}

	tm.mu.Lock()
	tm.currentToken = resp.AccessToken
	tm.tokenExpiry = time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
	tm.configURL = resp.ConfigURL
	tm.logsURL = resp.LogsURL
	tm.mu.Unlock()

	logger.Trace("Token refreshed successfully")

	// Check for configuration updates
	if manager := GetManager(); manager != nil {
		manager.CheckConfigUpdates(ctx)
	}

	return nil
}

// GetToken returns the current access token
func (tm *TokenManager) GetToken() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.currentToken
}

// GetConfigURL returns the config API URL
func (tm *TokenManager) GetConfigURL() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	url := tm.configURL
	if url == "" {
		logger.Debug("Config URL is empty")
	}
	return url
}

// GetLogsURL returns the logs URL
func (tm *TokenManager) GetLogsURL() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.logsURL
}

// IsDeploymentActive returns whether the deployment is active
func (tm *TokenManager) IsDeploymentActive() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return !tm.deploymentDeleted
}

// Stop stops the token manager
func (tm *TokenManager) Stop() {
	close(tm.stopCh)
}
