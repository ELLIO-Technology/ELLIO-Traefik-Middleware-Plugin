package api

// BootstrapResponse represents the bootstrap API response
type BootstrapResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ConfigURL   string `json:"config_url"`
	LogsURL     string `json:"logs_url,omitempty"`
}

// EDLConfig represents the EDL configuration
type EDLConfig struct {
	DeploymentID           string  `json:"deployment_id"`
	Purpose                string  `json:"purpose"` // "allowlist", "blocklist", "other"
	Direction              string  `json:"direction"`
	UpdateFrequencySeconds int     `json:"update_frequency_seconds"`
	FirewallFormat         string  `json:"firewall_format"`
	URLs                   EDLURLs `json:"urls"`
}

// EDLURLs contains the EDL URLs
type EDLURLs struct {
	Combined []string `json:"combined,omitempty"`
}

// APIError represents an API error response
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return e.Message
}

// IsPermanentError checks if an error is a permanent error (410)
func IsPermanentError(err error) bool {
	// Use type assertion instead of errors.As to avoid Yaegi issues
	apiErr, ok := err.(*APIError)
	if ok && apiErr.StatusCode == 410 {
		return true
	}
	return false
}

// IsTemporaryDisabled checks if an error indicates temporary deployment disabled (403)
func IsTemporaryDisabled(err error) bool {
	// Use type assertion instead of errors.As to avoid Yaegi issues
	apiErr, ok := err.(*APIError)
	if ok && apiErr.StatusCode == 403 {
		return true
	}
	return false
}
