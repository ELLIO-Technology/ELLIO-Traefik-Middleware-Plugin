package ELLIO_Traefik_Middleware_Plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateConfig(t *testing.T) {
	config := CreateConfig()
	if config == nil {
		t.Fatal("CreateConfig returned nil")
	}
}

func TestNew(t *testing.T) {
	// Skip network-dependent tests in short mode
	if testing.Short() {
		t.Skip("Skipping network-dependent test in short mode")
	}

	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "missing bootstrap token",
			config: &Config{
				LogLevel: "info",
			},
			expectError: true,
			errorMsg:    "bootstrap token is required",
		},
		{
			name: "invalid token format",
			config: &Config{
				BootstrapToken: "invalid-token",
				LogLevel:       "info",
			},
			expectError: true,
			errorMsg:    "", // Will fail with JWT parsing error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler, err := New(ctx, next, tt.config, "test-middleware")

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if handler == nil {
					t.Error("handler is nil")
				}
			}
		})
	}
}

func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		name           string
		remoteAddr     string
		headers        map[string]string
		ipStrategy     string
		trustedHeader  string
		trustedProxies []string
		expectedIP     string
	}{
		{
			name:       "direct strategy",
			remoteAddr: "192.168.1.1:12345",
			ipStrategy: "direct",
			expectedIP: "192.168.1.1",
		},
		{
			name:       "xff strategy with trusted proxy",
			remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1, 10.0.0.1",
			},
			ipStrategy:     "xff",
			trustedProxies: []string{"10.0.0.0/8"},
			expectedIP:     "203.0.113.1",
		},
		{
			name:       "xff strategy with untrusted proxy",
			remoteAddr: "192.168.1.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1",
			},
			ipStrategy:     "xff",
			trustedProxies: []string{"10.0.0.0/8"},
			expectedIP:     "192.168.1.1", // Falls back to direct IP
		},
		{
			name:       "real-ip strategy",
			remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.1",
			},
			ipStrategy:     "real-ip",
			trustedProxies: []string{"10.0.0.0/8"},
			expectedIP:     "203.0.113.1",
		},
		{
			name:       "custom header strategy",
			remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{
				"CF-Connecting-IP": "203.0.113.1",
			},
			ipStrategy:     "custom",
			trustedHeader:  "CF-Connecting-IP",
			trustedProxies: []string{"10.0.0.0/8"},
			expectedIP:     "203.0.113.1",
		},
		{
			name:           "loopback trusted proxy",
			remoteAddr:     "127.0.0.1:12345",
			headers:        map[string]string{"X-Forwarded-For": "203.0.113.1"},
			ipStrategy:     "xff",
			trustedProxies: []string{"loopback"},
			expectedIP:     "203.0.113.1",
		},
		{
			name:           "private trusted proxy",
			remoteAddr:     "192.168.1.1:12345",
			headers:        map[string]string{"X-Real-IP": "203.0.113.1"},
			ipStrategy:     "real-ip",
			trustedProxies: []string{"private"},
			expectedIP:     "203.0.113.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := &EllioMiddleware{
				config: &Config{
					IPStrategy:     tt.ipStrategy,
					TrustedHeader:  tt.trustedHeader,
					TrustedProxies: tt.trustedProxies,
				},
				trustedProxies: parseTrustedProxies(tt.trustedProxies),
			}

			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			ip := middleware.extractClientIP(req)
			if ip != tt.expectedIP {
				t.Errorf("expected IP %q, got %q", tt.expectedIP, ip)
			}
		})
	}
}

func TestParseTrustedProxies(t *testing.T) {
	tests := []struct {
		name     string
		proxies  []string
		expected int // Expected number of parsed prefixes
	}{
		{
			name:     "single IP",
			proxies:  []string{"192.168.1.1"},
			expected: 1,
		},
		{
			name:     "CIDR range",
			proxies:  []string{"10.0.0.0/8"},
			expected: 1,
		},
		{
			name:     "loopback keyword",
			proxies:  []string{"loopback"},
			expected: 2, // IPv4 and IPv6 loopback
		},
		{
			name:     "private keyword",
			proxies:  []string{"private"},
			expected: 5, // 3 IPv4 private ranges + 2 IPv6
		},
		{
			name:     "mixed",
			proxies:  []string{"192.168.1.1", "10.0.0.0/8", "loopback"},
			expected: 4,
		},
		{
			name:     "invalid entry",
			proxies:  []string{"invalid", "192.168.1.1"},
			expected: 1, // Only valid entry parsed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTrustedProxies(tt.proxies)
			if len(result) != tt.expected {
				t.Errorf("expected %d prefixes, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestGetDirectIP(t *testing.T) {
	tests := []struct {
		remoteAddr string
		expected   string
	}{
		{"192.168.1.1:8080", "192.168.1.1"},
		{"[::1]:8080", "::1"},
		{"192.168.1.1", "192.168.1.1"},
		{"invalid:multiple:colons", "invalid:multiple:colons"},
	}

	for _, tt := range tests {
		t.Run(tt.remoteAddr, func(t *testing.T) {
			result := getDirectIP(tt.remoteAddr)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestServeHTTP_WithoutManager(t *testing.T) {
	// Test when singleton manager is not initialized
	middleware := &EllioMiddleware{
		next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}),
		name:   "test",
		config: &Config{},
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "OK" {
		t.Errorf("expected body 'OK', got %q", rec.Body.String())
	}
}

func TestServeHTTP_PanicRecovery(t *testing.T) {
	// Test panic recovery
	middleware := &EllioMiddleware{
		next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		}),
		name:   "test",
		config: &Config{IPStrategy: "direct"},
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ServeHTTP did not recover from panic: %v", r)
		}
	}()

	middleware.ServeHTTP(rec, req)

	// Should return 500 after panic recovery
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 after panic, got %d", rec.Code)
	}
}
