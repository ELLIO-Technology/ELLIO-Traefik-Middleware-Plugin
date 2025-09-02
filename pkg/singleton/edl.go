package singleton

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/ipmatcher"
	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/iptrie"
	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/logger"
)

// EDLUpdater manages EDL fetching and updating
type EDLUpdater struct {
	url             string
	updateFrequency time.Duration
	matcher         *ipmatcher.Matcher
	client          *http.Client
	manager         *Manager // Reference to manager for cache clearing

	mu          sync.RWMutex
	lastUpdate  time.Time
	lastError   error
	updateCount int64

	stopCh        chan struct{}
	reconfigureCh chan struct{} // Signal to restart update loop
}

// NewEDLUpdater creates a new EDL updater
func NewEDLUpdater(url string, updateFrequency time.Duration, matcher *ipmatcher.Matcher, manager *Manager) *EDLUpdater {
	return &EDLUpdater{
		url:             url,
		updateFrequency: updateFrequency,
		matcher:         matcher,
		manager:         manager,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  true,
				MaxIdleConnsPerHost: 2,
			},
		},
		stopCh:        make(chan struct{}),
		reconfigureCh: make(chan struct{}, 1),
	}
}

// Start performs initial EDL fetch
func (u *EDLUpdater) Start(ctx context.Context) error {
	if u.url == "" {
		return errors.New("EDL URL is empty")
	}

	logger.Debug("Loading initial EDL data...")
	if err := u.updateNow(ctx); err != nil {
		return errors.New("initial EDL fetch failed: " + err.Error())
	}

	return nil
}

// StartUpdateLoop starts the background update loop
func (u *EDLUpdater) StartUpdateLoop(ctx context.Context) {
	for {
		u.mu.RLock()
		freq := u.updateFrequency
		u.mu.RUnlock()

		ticker := time.NewTicker(freq)

		// Inner loop with current configuration
		running := true
		for running {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-u.stopCh:
				ticker.Stop()
				return
			case <-u.reconfigureCh:
				// Configuration changed, restart with new settings
				ticker.Stop()
				running = false
				logger.Trace("EDL updater reconfiguring with new settings")
			case <-ticker.C:
				if err := u.updateNow(ctx); err != nil {
					logger.Errorf("EDL update failed: %v", err)
				}
			}
		}
	}
}

// updateNow performs an immediate EDL update
func (u *EDLUpdater) updateNow(ctx context.Context) error {
	start := time.Now()

	trie, count, err := u.fetchWithRetry(ctx)
	if err != nil {
		u.mu.Lock()
		u.lastError = err
		u.mu.Unlock()
		return err
	}

	// Update the matcher
	u.matcher.Update(trie, count)

	u.mu.Lock()
	u.lastUpdate = time.Now()
	u.lastError = nil
	u.updateCount++
	u.mu.Unlock()

	duration := time.Since(start)
	if count == 0 {
		logger.Infof("EDL updated with empty list in %v", duration)
	} else {
		deploymentID := ""
		if u.manager != nil {
			deploymentID = u.manager.deploymentID
		}
		if deploymentID != "" {
			logger.Infof("EDL loaded for deployment %s in %v", deploymentID, duration)
		} else {
			logger.Infof("EDL loaded in %v", duration)
		}
		logger.Tracef("EDL approximate entry count: %d", count)
	}

	return nil
}

// fetchWithRetry fetches EDL with retry logic
func (u *EDLUpdater) fetchWithRetry(ctx context.Context) (*iptrie.Trie, int64, error) {
	var lastErr error
	maxAttempts := 3

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}

		trie, count, err := u.fetch(ctx)
		if err == nil {
			return trie, count, nil
		}

		lastErr = err
		logger.Warnf("EDL fetch attempt %d/%d failed: %v", attempt+1, maxAttempts, err)
	}

	return nil, 0, lastErr
}

// fetch performs a single EDL fetch
func (u *EDLUpdater) fetch(ctx context.Context) (*iptrie.Trie, int64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u.url, nil)
	if err != nil {
		return nil, 0, err
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, 0, errors.New("unexpected status: " + string(body))
	}

	return u.parseEDL(resp.Body)
}

// parseEDL parses the EDL response (binary format only)
func (u *EDLUpdater) parseEDL(r io.Reader) (*iptrie.Trie, int64, error) {
	// Fast binary format parsing
	trie, count, err := iptrie.LoadBinaryTrie(r)
	if err != nil {
		return nil, 0, err
	}

	if count == 0 {
		logger.Warn("EDL is empty - no IP addresses found")
	}

	return trie, count, nil
}

// GetStatus returns the current status
func (u *EDLUpdater) GetStatus() (time.Time, error, int64) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.lastUpdate, u.lastError, u.updateCount
}

// Reconfigure updates the EDL URL and update frequency
func (u *EDLUpdater) Reconfigure(url string, updateFrequency time.Duration) {
	u.mu.Lock()
	defer u.mu.Unlock()

	// Update configuration
	u.url = url
	u.updateFrequency = updateFrequency

	// Signal the update loop to restart with new settings
	select {
	case u.reconfigureCh <- struct{}{}:
		// Signal sent
	default:
		// Channel already has a signal, that's fine
	}

	// Trigger immediate update with new URL
	go func() {
		if err := u.updateNow(context.Background()); err != nil {
			logger.Errorf("EDL update after reconfiguration failed: %v", err)
		}
	}()
}

// Stop stops the updater
func (u *EDLUpdater) Stop() {
	close(u.stopCh)
}
