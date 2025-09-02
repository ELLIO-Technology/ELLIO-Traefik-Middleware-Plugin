package logs

import (
	"net/http"
	"testing"
	"time"
)

func TestNewBlockEvent(t *testing.T) {
	event := NewBlockEvent(
		"192.168.1.1", // extractedIP
		"10.0.0.1",    // directIP
		"GET",         // method
		"example.com", // host
		"/api/test",   // path
		"https",       // scheme
		"Mozilla/5.0", // userAgent
		"blocklist",   // edlMode
	)

	if event == nil {
		t.Fatal("NewBlockEvent returned nil")
	}

	// Check basic fields
	if event.EventType != "access_blocked" {
		t.Errorf("expected EventType 'access_blocked', got %s", event.EventType)
	}

	if event.StatusCode != http.StatusForbidden {
		t.Errorf("expected StatusCode %d, got %d", http.StatusForbidden, event.StatusCode)
	}

	// Check request details
	if event.Request.Method != "GET" {
		t.Errorf("expected Method 'GET', got %s", event.Request.Method)
	}

	if event.Request.Host != "example.com" {
		t.Errorf("expected Host 'example.com', got %s", event.Request.Host)
	}

	if event.Request.Path != "/api/test" {
		t.Errorf("expected Path '/api/test', got %s", event.Request.Path)
	}

	if event.Request.Scheme != "https" {
		t.Errorf("expected Scheme 'https', got %s", event.Request.Scheme)
	}

	// Check client info
	if event.Client.IP != "192.168.1.1" {
		t.Errorf("expected Client.IP '192.168.1.1', got %s", event.Client.IP)
	}

	if event.Client.DirectIP != "10.0.0.1" {
		t.Errorf("expected Client.DirectIP '10.0.0.1', got %s", event.Client.DirectIP)
	}

	if event.Client.UserAgent != "Mozilla/5.0" {
		t.Errorf("expected UserAgent 'Mozilla/5.0', got %s", event.Client.UserAgent)
	}

	// Check policy
	if event.Policy.Mode != "blocklist" {
		t.Errorf("expected Policy.Mode 'blocklist', got %s", event.Policy.Mode)
	}

	// Check timestamp is recent
	if time.Since(event.Timestamp) > 1*time.Second {
		t.Error("Timestamp is not recent")
	}
}

func TestReturnToPool(t *testing.T) {
	event := NewBlockEvent(
		"192.168.1.1",
		"10.0.0.1",
		"POST",
		"test.com",
		"/test",
		"http",
		"TestAgent",
		"allowlist",
	)

	// Return event to pool
	ReturnToPool(event)

	// Check that sensitive data is cleared
	if event.Client.IP != "" {
		t.Error("Client.IP should be cleared")
	}

	if event.Client.DirectIP != "" {
		t.Error("Client.DirectIP should be cleared")
	}

	if event.Client.UserAgent != "" {
		t.Error("Client.UserAgent should be cleared")
	}

	if event.Request.Host != "" {
		t.Error("Request.Host should be cleared")
	}

	if event.Request.Path != "" {
		t.Error("Request.Path should be cleared")
	}
}

func TestEventPool(t *testing.T) {
	// Create multiple events to test pool reuse
	events := make([]*BlockEvent, 10)
	for i := 0; i < 10; i++ {
		events[i] = NewBlockEvent(
			"192.168.1.1",
			"10.0.0.1",
			"GET",
			"example.com",
			"/",
			"http",
			"",
			"blocklist",
		)
	}

	// Return all to pool
	for _, e := range events {
		ReturnToPool(e)
	}

	// Get new events - should reuse from pool
	for i := 0; i < 5; i++ {
		event := NewBlockEvent(
			"10.0.0.1",
			"127.0.0.1",
			"POST",
			"test.com",
			"/api",
			"https",
			"TestAgent",
			"allowlist",
		)

		if event == nil {
			t.Error("Failed to get event from pool")
			continue
		}

		// Verify it's properly initialized
		if event.Client.IP != "10.0.0.1" {
			t.Error("Event not properly reinitialized")
		}
	}
}

func BenchmarkNewBlockEvent(b *testing.B) {
	for i := 0; i < b.N; i++ {
		event := NewBlockEvent(
			"192.168.1.1",
			"10.0.0.1",
			"GET",
			"example.com",
			"/api/test",
			"https",
			"Mozilla/5.0",
			"blocklist",
		)
		ReturnToPool(event)
	}
}

func BenchmarkNewBlockEventWithoutPool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = &BlockEvent{
			Timestamp:  time.Now().UTC(),
			EventType:  "access_blocked",
			StatusCode: http.StatusForbidden,
			Request: RequestDetails{
				Method: "GET",
				Host:   "example.com",
				Path:   "/api/test",
				Scheme: "https",
			},
			Client: ClientInfo{
				IP:        "192.168.1.1",
				DirectIP:  "10.0.0.1",
				UserAgent: "Mozilla/5.0",
			},
			Policy: PolicyInfo{
				Mode: "blocklist",
			},
		}
	}
}
