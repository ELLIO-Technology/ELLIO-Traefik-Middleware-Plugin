package singleton

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"time"

	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/api"
	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/ipmatcher"
	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/logger"
	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/logs"
	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/utils"
)

var (
	instance *Manager
	once     sync.Once
	initErr  error
)

type Manager struct {
	mu                  sync.RWMutex
	bootstrapToken      string
	tokenManager        *TokenManager
	edlUpdater          *EDLUpdater
	matcher             *ipmatcher.Matcher
	logShipper          *logs.LogShipper
	deploymentEnabled   bool
	temporarilyDisabled bool          // True when deployment is temporarily disabled (403)
	disabledCheckTime   time.Time     // Next time to check if deployment is re-enabled
	edlMode             string        // "blocklist" or "allowlist"
	edlURL              string        // Current EDL URL
	edlUpdateFreq       time.Duration // Current update frequency
	deviceID            string
	deploymentID        string // Deployment ID from JWT
	stopCh              chan struct{}
	disabledRetryCh     chan struct{} // Channel to trigger retry for disabled deployment
}

// Initialize creates and starts the singleton manager
func Initialize(bootstrapToken, machineID string, ipStrategy string, trustedHeader string, trustedProxies []string) error {
	logger.Trace("Initialize called")
	once.Do(func() {
		logger.Trace("Inside once.Do")
		if bootstrapToken == "" {
			logger.Error("Bootstrap token is empty")
			initErr = errors.New("bootstrap token is required")
			return
		}

		logger.Trace("Creating manager instance")
		manager := &Manager{
			bootstrapToken:  bootstrapToken,
			matcher:         ipmatcher.New(),
			stopCh:          make(chan struct{}),
			disabledRetryCh: make(chan struct{}, 1),
		}

		// Set instance early to avoid race condition
		// Even if initialization fails later, we have a valid (but disabled) manager
		logger.Trace("Setting global instance")
		instance = manager

		// Use provided machine ID or generate random one
		if machineID != "" {
			manager.deviceID = machineID
			logger.Infof("Using provided machine ID: %s", machineID)
		} else {
			manager.deviceID = utils.GenerateMachineID()
			logger.Infof("Generated random machine ID: %s", manager.deviceID)
		}

		// Initialize token manager
		manager.tokenManager = NewTokenManager(bootstrapToken, manager.deviceID)

		// Parse JWT to validate component_type and issuer
		claims, err := manager.tokenManager.ParseBootstrapToken()
		if err != nil {
			initErr = err
			return
		}

		// Store deployment ID
		manager.deploymentID = claims.DeploymentID

		// Validate component type
		if claims.ComponentType != "ellio_traefik_middleware_plugin" {
			initErr = errors.New("invalid component_type in JWT, expected ellio_traefik_middleware_plugin")
			return
		}

		// Validate issuer is present (required for bootstrap URL construction)
		if claims.Issuer == "" {
			initErr = errors.New("bootstrap token missing issuer")
			return
		}

		// Initialize with bootstrap (30 second timeout is fine for bootstrap)
		if manager.deploymentID != "" {
			logger.Infof("Initializing ELLIO middleware for deployment: %s", manager.deploymentID)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := manager.tokenManager.Initialize(ctx); err != nil {
			if api.IsPermanentError(err) {
				// Deployment deleted, run in allow-all mode
				manager.deploymentEnabled = false
				logger.Info("Deployment deleted (410), running in allow-all mode")
			} else if api.IsTemporaryDisabled(err) {
				// Deployment temporarily disabled, run in allow-all mode but retry
				manager.temporarilyDisabled = true
				manager.disabledCheckTime = time.Now().Add(1 * time.Minute)
				logger.Info("Deployment temporarily disabled (403), running in allow-all mode, will retry in 1 minute")
				// Start retry goroutine
				go manager.startDisabledRetryLoop()
			} else {
				initErr = err
				return
			}
		}

		// Initialize log shipper if we have a logs URL
		if logsURL := manager.tokenManager.GetLogsURL(); logsURL != "" {
			logger.Debugf("Initializing log shipper with URL: %s", logsURL)
			logConfig := &logs.LogShipperConfig{
				BatchSize:      100,
				FlushInterval:  1 * time.Second,
				BucketCapacity: 1000,
				RefillRate:     100,
				BufferSize:     10000,
			}
			manager.logShipper = logs.NewLogShipper(manager.tokenManager, logConfig)

			// Set batch metadata
			metadata := &logs.BatchMetadata{
				DeviceID:   manager.deviceID,
				IPStrategy: ipStrategy,
			}
			// Only include optional fields if configured
			if ipStrategy == "custom" && trustedHeader != "" {
				metadata.TrustedHeader = trustedHeader
			}
			if len(trustedProxies) > 0 {
				metadata.TrustedProxies = trustedProxies
			}
			manager.logShipper.SetBatchMetadata(metadata)

			manager.logShipper.Start()
			logger.Debug("Log shipper initialized and started")
		} else {
			logger.Trace("No logs URL available, log shipper not initialized")
		}

		if manager.deploymentEnabled = manager.tokenManager.IsDeploymentActive(); manager.deploymentEnabled {
			// Use longer timeout for EDL operations (Yaegi is slower than native Go)
			edlCtx := context.Background() // No timeout for EDL parsing in Yaegi

			// Fetch EDL configuration
			logger.Debugf("Fetching EDL configuration for deployment: %s", manager.deploymentID)
			edlConfig, err := manager.fetchEDLConfig(edlCtx)
			if err != nil {
				if api.IsPermanentError(err) {
					manager.deploymentEnabled = false
					logger.Info("Deployment deleted while fetching config")
				} else if api.IsTemporaryDisabled(err) {
					manager.temporarilyDisabled = true
					manager.disabledCheckTime = time.Now().Add(1 * time.Minute)
					logger.Info("Deployment temporarily disabled while fetching config")
					go manager.startDisabledRetryLoop()
				} else {
					logger.Errorf("Failed to fetch EDL config: %v", err)
					initErr = err
					return
				}
			}

			// EDL is enabled if we have a valid config with URLs
			if manager.deploymentEnabled && edlConfig != nil && len(edlConfig.URLs.Combined) > 0 {
				// Set EDL mode
				switch edlConfig.Purpose {
				case "allowlist":
					manager.edlMode = "allowlist"
				case "blocklist", "other", "others":
					manager.edlMode = "blocklist"
				default:
					manager.edlMode = "blocklist"
				}

				// Initialize EDL updater
				var edlURL string
				if len(edlConfig.URLs.Combined) > 0 {
					edlURL = edlConfig.URLs.Combined[0]
				}

				updateFreq := time.Duration(edlConfig.UpdateFrequencySeconds) * time.Second
				if updateFreq <= 0 {
					updateFreq = 5 * time.Minute
				}

				// Store current configuration
				manager.edlURL = edlURL
				manager.edlUpdateFreq = updateFreq

				manager.edlUpdater = NewEDLUpdater(edlURL, updateFreq, manager.matcher, manager)

				// Start EDL updater (use edlCtx without timeout for Yaegi)
				logger.Debugf("Starting EDL updater for deployment: %s", manager.deploymentID)
				if err := manager.edlUpdater.Start(edlCtx); err != nil {
					logger.Errorf("Failed to start EDL updater: %v", err)
					initErr = err
					return
				}
				logger.Debug("EDL updater started successfully")

				// Start background refresh loops
				go manager.tokenManager.StartRefreshLoop(context.Background())
				go manager.edlUpdater.StartUpdateLoop(context.Background())
			} else {
				manager.deploymentEnabled = false
			}
		}
		logger.Tracef("Initialization complete - deploymentEnabled=%v", manager.deploymentEnabled)
	})

	logger.Tracef("Initialize returning - err=%v", initErr)
	return initErr
}

// GetManager returns the singleton manager instance
func GetManager() *Manager {
	return instance
}

// IsDeploymentEnabled returns whether deployment is enabled
func (m *Manager) IsDeploymentEnabled() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.deploymentEnabled && !m.temporarilyDisabled
}

// IsIPAllowed checks if an IP is allowed based on EDL
func (m *Manager) IsIPAllowed(clientIP string) (bool, error) {
	// If deployment is disabled, allow all (check without lock)
	if !m.IsDeploymentEnabled() {
		return true, nil
	}

	// Check against EDL directly (no cache)
	inList := m.matcher.Contains(clientIP)

	// XOR operation: allowed if (blocklist AND NOT in list) OR (allowlist AND in list)
	m.mu.RLock()
	isBlocklist := m.edlMode == "blocklist"
	m.mu.RUnlock()

	allowed := isBlocklist != inList
	return allowed, nil
}

// IsIPAllowedWithStats checks if an IP is allowed and returns timing stats
func (m *Manager) IsIPAllowedWithStats(clientIP string) (bool, bool, error) {
	// If deployment is disabled, allow all (check without lock)
	if !m.IsDeploymentEnabled() {
		return true, false, nil
	}

	var debugMode = logger.IsDebugEnabled()
	var timings = make(map[string]time.Duration)
	var overallStart time.Time
	if debugMode {
		overallStart = time.Now()
	}

	// Check against EDL directly (no cache)

	// Parse IP address
	var parseStart time.Time
	if debugMode {
		parseStart = time.Now()
	}
	addr, err := netip.ParseAddr(clientIP)
	if err != nil {
		return false, false, err
	}
	if debugMode {
		timings["parse"] = time.Since(parseStart)
	}

	// Check against EDL
	var lookupStart time.Time
	if debugMode {
		lookupStart = time.Now()
	}
	inList := m.matcher.ContainsAddr(addr)
	if debugMode {
		timings["lookup"] = time.Since(lookupStart)
	}

	// XOR operation: allowed if (blocklist AND NOT in list) OR (allowlist AND in list)
	var modeCheckStart time.Time
	if debugMode {
		modeCheckStart = time.Now()
	}
	m.mu.RLock()
	isBlocklist := m.edlMode == "blocklist"
	m.mu.RUnlock()
	if debugMode {
		timings["mode_check"] = time.Since(modeCheckStart)
	}

	var logicStart time.Time
	if debugMode {
		logicStart = time.Now()
	}
	allowed := isBlocklist != inList
	if debugMode {
		timings["logic"] = time.Since(logicStart)
	}

	// Log timing breakdown
	if debugMode {
		total := time.Since(overallStart)
		logger.Debugf("IP_CHECK %s - total=%v [parse=%v, lookup=%v, mode_check=%v, logic=%v]",
			clientIP, total, timings["parse"], timings["lookup"], timings["mode_check"], timings["logic"])
	}

	return allowed, false, nil // false = no cache anymore
}

// fetchEDLConfig fetches the EDL configuration from the API
func (m *Manager) fetchEDLConfig(ctx context.Context) (*api.EDLConfig, error) {
	configURL := m.tokenManager.GetConfigURL()
	logger.Tracef("Fetching EDL config from URL: %s", configURL)

	configClient := api.NewConfigClient(configURL, m.tokenManager.GetToken)

	edlConfig, err := configClient.GetEDLConfig(ctx)
	if err != nil {
		logger.Errorf("Failed to get EDL config: %v", err)
		return nil, err
	}

	logger.Infof("EDL configuration for deployment %s: mode=%s",
		m.deploymentID, edlConfig.Purpose)
	return edlConfig, nil
}

// SendBlockEvent sends a block event to the log shipper
func (m *Manager) SendBlockEvent(event *logs.BlockEvent) {
	if m.logShipper != nil {
		logger.Tracef("Sending block event to log shipper - ip=%s directIP=%s",
			event.Client.IP, event.Client.DirectIP)
		m.logShipper.SendEvent(event)
	} else {
		logger.Trace("Log shipper is nil, cannot send event")
	}
}

// GetDeviceID returns the device ID
func (m *Manager) GetDeviceID() string {
	return m.deviceID
}

// GetEDLMode returns the current EDL mode
func (m *Manager) GetEDLMode() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.edlMode
}

// CheckConfigUpdates fetches and applies any configuration changes
func (m *Manager) CheckConfigUpdates(ctx context.Context) {
	// Only check if deployment is enabled
	if !m.IsDeploymentEnabled() {
		return
	}

	// Fetch current EDL config
	edlConfig, err := m.fetchEDLConfig(ctx)
	if err != nil {
		if api.IsPermanentError(err) {
			m.mu.Lock()
			m.deploymentEnabled = false
			m.mu.Unlock()
			logger.Info("Deployment deleted during config check")
		} else if api.IsTemporaryDisabled(err) {
			m.mu.Lock()
			m.temporarilyDisabled = true
			m.disabledCheckTime = time.Now().Add(1 * time.Minute)
			m.mu.Unlock()
			logger.Info("Deployment temporarily disabled during config check, will retry in 1 minute")
		}
		return // Keep using current config on error
	}

	// Check if we have valid EDL config
	if edlConfig == nil || len(edlConfig.URLs.Combined) == 0 {
		return
	}

	// Extract new configuration
	var newURL string
	if len(edlConfig.URLs.Combined) > 0 {
		newURL = edlConfig.URLs.Combined[0]
	}

	newUpdateFreq := time.Duration(edlConfig.UpdateFrequencySeconds) * time.Second
	if newUpdateFreq <= 0 {
		newUpdateFreq = 5 * time.Minute
	}

	newMode := "blocklist"
	switch edlConfig.Purpose {
	case "allowlist":
		newMode = "allowlist"
	case "blocklist", "other", "others":
		newMode = "blocklist"
	}

	// Check if configuration changed
	m.mu.Lock()
	urlChanged := m.edlURL != newURL
	freqChanged := m.edlUpdateFreq != newUpdateFreq
	modeChanged := m.edlMode != newMode
	m.mu.Unlock()

	if !urlChanged && !freqChanged && !modeChanged {
		return // No changes
	}

	// Log configuration changes
	if urlChanged {
		logger.Infof("EDL URL changed from %s to %s", m.edlURL, newURL)
	}
	if freqChanged {
		logger.Infof("EDL update frequency changed from %v to %v", m.edlUpdateFreq, newUpdateFreq)
	}
	if modeChanged {
		logger.Infof("EDL mode changed from %s to %s", m.edlMode, newMode)
	}

	// Update configuration
	m.mu.Lock()
	m.edlURL = newURL
	m.edlUpdateFreq = newUpdateFreq
	m.edlMode = newMode
	m.mu.Unlock()

	// Mode changed - no cache to clear anymore

	// Reconfigure EDL updater
	if m.edlUpdater != nil {
		m.edlUpdater.Reconfigure(newURL, newUpdateFreq)
	}
}

// Stop gracefully stops the manager
func (m *Manager) Stop() {
	close(m.stopCh)
	if m.tokenManager != nil {
		m.tokenManager.Stop()
	}
	if m.edlUpdater != nil {
		m.edlUpdater.Stop()
	}
	if m.logShipper != nil {
		if err := m.logShipper.Stop(); err != nil {
			logger.Errorf("Error stopping log shipper: %v", err)
		}
	}
}

// startDisabledRetryLoop starts a goroutine that retries when deployment is temporarily disabled
func (m *Manager) startDisabledRetryLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.mu.RLock()
			shouldRetry := m.temporarilyDisabled && time.Now().After(m.disabledCheckTime)
			m.mu.RUnlock()

			if !shouldRetry {
				continue
			}

			logger.Info("Retrying to check if deployment is re-enabled...")

			// Try to reinitialize
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			err := m.tokenManager.Initialize(ctx)
			cancel()

			if err == nil {
				// Success - deployment is re-enabled
				m.mu.Lock()
				m.temporarilyDisabled = false
				m.deploymentEnabled = true
				m.mu.Unlock()

				logger.Info("Deployment re-enabled successfully")

				// Fetch EDL config and reinitialize
				ctx := context.Background()
				edlConfig, err := m.fetchEDLConfig(ctx)
				if err == nil && edlConfig != nil && len(edlConfig.URLs.Combined) > 0 {
					// Reinitialize EDL
					m.mu.Lock()
					switch edlConfig.Purpose {
					case "allowlist":
						m.edlMode = "allowlist"
					default:
						m.edlMode = "blocklist"
					}

					if len(edlConfig.URLs.Combined) > 0 {
						m.edlURL = edlConfig.URLs.Combined[0]
					}

					m.edlUpdateFreq = time.Duration(edlConfig.UpdateFrequencySeconds) * time.Second
					if m.edlUpdateFreq <= 0 {
						m.edlUpdateFreq = 5 * time.Minute
					}
					m.mu.Unlock()

					// Restart EDL updater if needed
					if m.edlUpdater != nil {
						m.edlUpdater.Reconfigure(m.edlURL, m.edlUpdateFreq)
						go m.edlUpdater.StartUpdateLoop(context.Background())
					} else if m.edlURL != "" {
						// Create new EDL updater
						m.edlUpdater = NewEDLUpdater(m.edlURL, m.edlUpdateFreq, m.matcher, m)
						if err := m.edlUpdater.Start(context.Background()); err == nil {
							go m.edlUpdater.StartUpdateLoop(context.Background())
						}
					}
				}

				return // Exit retry loop
			} else if api.IsPermanentError(err) {
				// Deployment was deleted
				m.mu.Lock()
				m.temporarilyDisabled = false
				m.deploymentEnabled = false
				m.mu.Unlock()
				logger.Info("Deployment deleted (410) during retry")
				return // Exit retry loop
			} else if api.IsTemporaryDisabled(err) {
				// Still disabled, update check time
				m.mu.Lock()
				m.disabledCheckTime = time.Now().Add(1 * time.Minute)
				m.mu.Unlock()
				logger.Trace("Deployment still disabled, will retry again in 1 minute")
			} else {
				// Other error, retry in 1 minute
				m.mu.Lock()
				m.disabledCheckTime = time.Now().Add(1 * time.Minute)
				m.mu.Unlock()
				logger.Errorf("Error checking deployment status: %v, will retry in 1 minute", err)
			}
		}
	}
}
