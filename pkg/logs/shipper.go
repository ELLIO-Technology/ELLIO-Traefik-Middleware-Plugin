package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/logger"
)

const (
	defaultBatchSize     = 1000
	defaultFlushInterval = 10 * time.Second
	maxRetries           = 3
	initialBackoff       = 1 * time.Second
	maxBackoff           = 10 * time.Second
)

// TokenProvider provides access token and logs URL
type TokenProvider interface {
	GetToken() string
	GetLogsURL() string
}

// BatchMetadata contains metadata about the middleware configuration
type BatchMetadata struct {
	DeviceID       string   `json:"device_id"`
	IPStrategy     string   `json:"ip_strategy,omitempty"`     // "direct", "xff", "real-ip", "custom"
	TrustedHeader  string   `json:"trusted_header,omitempty"`  // Only if strategy is "custom"
	TrustedProxies []string `json:"trusted_proxies,omitempty"` // Only if configured
}

// BatchPayload wraps events with metadata
type BatchPayload struct {
	BatchMetadata *BatchMetadata `json:"batch_metadata"`
	Events        []*BlockEvent  `json:"events"`
}

// LogShipper handles batching and shipping of events
type LogShipper struct {
	client        *http.Client
	tokenProvider TokenProvider
	bucket        *LeakyBucket

	eventChan chan *BlockEvent
	buffer    *RingBuffer

	batchSize     int
	flushInterval time.Duration

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	// Batch metadata
	batchMetadata *BatchMetadata
	metaMu        sync.RWMutex

	// Stats
	eventsShipped int64
	eventsDropped int64
	mu            sync.Mutex
}

// LogShipperConfig holds configuration for the log shipper
type LogShipperConfig struct {
	BatchSize      int
	FlushInterval  time.Duration
	BucketCapacity int64
	RefillRate     int64
	BufferSize     int
}

// SetBatchMetadata updates the batch metadata for all future shipments
func (s *LogShipper) SetBatchMetadata(metadata *BatchMetadata) {
	s.metaMu.Lock()
	s.batchMetadata = metadata
	s.metaMu.Unlock()
}

// NewLogShipper creates a new log shipper
func NewLogShipper(tokenProvider TokenProvider, config *LogShipperConfig) *LogShipper {
	if config.BatchSize <= 0 {
		config.BatchSize = defaultBatchSize
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = defaultFlushInterval
	}
	if config.BucketCapacity <= 0 {
		config.BucketCapacity = 10000
	}
	if config.RefillRate <= 0 {
		config.RefillRate = 100
	}
	if config.BufferSize <= 0 {
		config.BufferSize = 10000
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &LogShipper{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				MaxIdleConnsPerHost: 2,
			},
		},
		tokenProvider: tokenProvider,
		bucket:        NewLeakyBucket(config.BucketCapacity, config.RefillRate),
		eventChan:     make(chan *BlockEvent, 1000),
		buffer:        NewRingBuffer(config.BufferSize),
		batchSize:     config.BatchSize,
		flushInterval: config.FlushInterval,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start begins processing events
func (s *LogShipper) Start() {
	logger.Trace("Starting log shipper")
	s.wg.Add(1)
	go s.processEvents()
}

// Stop gracefully stops the shipper
func (s *LogShipper) Stop() error {
	s.cancel()
	close(s.eventChan)

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.flushBuffer()
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("timeout waiting for log shipper to stop")
	}
}

// SendEvent sends an event for shipping
func (s *LogShipper) SendEvent(event *BlockEvent) {
	select {
	case s.eventChan <- event:
		// Event sent successfully
	default:
		// Channel full, add to buffer
		if !s.buffer.Add(event) {
			s.mu.Lock()
			s.eventsDropped++
			dropped := s.eventsDropped
			s.mu.Unlock()
			logger.Warnf("Event dropped - buffer full (total dropped: %d)", dropped)
		}
	}
}

// processEvents handles batching and shipping
func (s *LogShipper) processEvents() {
	defer s.wg.Done()

	logger.Tracef("Log shipper goroutine started - batchSize=%d flushInterval=%v",
		s.batchSize, s.flushInterval)

	flushTicker := time.NewTicker(s.flushInterval)
	defer flushTicker.Stop()

	// Add a fast ticker to check for events more frequently
	checkTicker := time.NewTicker(100 * time.Millisecond)
	defer checkTicker.Stop()

	batch := make([]*BlockEvent, 0, s.batchSize)

	for {
		select {
		case <-s.ctx.Done():
			if len(batch) > 0 {
				s.shipBatch(batch)
			}
			return

		case event := <-s.eventChan:
			batch = append(batch, event)

			if len(batch) >= s.batchSize {
				s.shipBatch(batch)
				batch = make([]*BlockEvent, 0, s.batchSize)
			}

		case <-flushTicker.C:
			if len(batch) > 0 {
				s.shipBatch(batch)
				batch = make([]*BlockEvent, 0, s.batchSize)
			}
			// Process buffered events
			s.processBufferedEvents()

		case <-checkTicker.C:
			// Try to read events directly - workaround for Yaegi channel issues
			for i := 0; i < 100; i++ {
				select {
				case event, ok := <-s.eventChan:
					if !ok {
						if len(batch) > 0 {
							s.shipBatch(batch)
						}
						return
					}

					batch = append(batch, event)

					if len(batch) >= s.batchSize {
						s.shipBatch(batch)
						batch = make([]*BlockEvent, 0, s.batchSize)
					}
				default:
					break
				}
			}
		}
	}
}

// processBufferedEvents drains and ships buffered events
func (s *LogShipper) processBufferedEvents() {
	events := s.buffer.Drain(s.batchSize)
	if len(events) > 0 {
		s.shipBatch(events)
	}
}

// shipBatch sends a batch of events
func (s *LogShipper) shipBatch(events []*BlockEvent) {
	logger.Tracef("Shipping batch of %d events", len(events))

	// Rate limiting
	waitTime := s.bucket.WaitTime(1)
	if waitTime > 0 {
		logger.Tracef("Rate limited, waiting %v", waitTime)
		time.Sleep(waitTime)
	}

	if !s.bucket.Allow(1) {
		// Rate limited, re-buffer events
		logger.Warn("Rate limited, re-buffering events")
		for _, event := range events {
			if !s.buffer.Add(event) {
				s.mu.Lock()
				s.eventsDropped++
				s.mu.Unlock()
				ReturnToPool(event) // Return to pool if dropped
			}
		}
		return
	}

	// Convert to JSON payload with metadata
	payload, err := s.eventsToJSON(events)
	if err != nil {
		logger.Errorf("Failed to convert events to JSON: %v", err)
		s.mu.Lock()
		s.eventsDropped += int64(len(events))
		s.mu.Unlock()
		// Return events to pool
		for _, event := range events {
			ReturnToPool(event)
		}
		return
	}

	// Send with retry
	err = s.sendWithRetry(payload)
	if err != nil {
		logger.Warnf("Failed to ship batch of %d events: %v", len(events), err)
		// Re-buffer failed events
		for _, event := range events {
			if !s.buffer.Add(event) {
				s.mu.Lock()
				s.eventsDropped++
				s.mu.Unlock()
				ReturnToPool(event) // Return to pool if dropped
			}
		}
	} else {
		s.mu.Lock()
		s.eventsShipped += int64(len(events))
		shipped := s.eventsShipped
		s.mu.Unlock()
		logger.Debugf("Successfully shipped %d events (total: %d)", len(events), shipped)
		// Return successfully shipped events to pool
		for _, event := range events {
			ReturnToPool(event)
		}
	}
}

// sendWithRetry attempts to send payload with exponential backoff
func (s *LogShipper) sendWithRetry(payload []byte) error {
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(backoff)
			backoff = minDuration(backoff*2, maxBackoff)
		}

		err := s.send(payload)
		if err == nil {
			return nil
		}

		lastErr = err
	}

	return lastErr
}

// send performs the actual HTTP request
func (s *LogShipper) send(payload []byte) error {
	logsURL := s.tokenProvider.GetLogsURL()
	if logsURL == "" {
		return errors.New("logs URL not available")
	}

	token := s.tokenProvider.GetToken()
	if token == "" {
		return errors.New("access token not available")
	}

	req, err := http.NewRequestWithContext(s.ctx, "POST", logsURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return errors.New("server responded with: " + string(bodyBytes))
}

// flushBuffer sends all buffered events
func (s *LogShipper) flushBuffer() {
	events := s.buffer.DrainAll()

	for len(events) > 0 {
		batchSize := minInt(len(events), s.batchSize)
		batch := events[:batchSize]
		events = events[batchSize:]

		s.shipBatch(batch)
	}
}

// eventsToJSON converts events to JSON payload with metadata
func (s *LogShipper) eventsToJSON(events []*BlockEvent) ([]byte, error) {
	s.metaMu.RLock()
	metadata := s.batchMetadata
	s.metaMu.RUnlock()

	payload := BatchPayload{
		BatchMetadata: metadata,
		Events:        events,
	}

	return json.Marshal(payload)
}

// GetStats returns shipping statistics
func (s *LogShipper) GetStats() (shipped, dropped int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.eventsShipped, s.eventsDropped
}

// minDuration returns the minimum of two durations
func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
