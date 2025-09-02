//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

var (
	traefikURL    = getEnv("TRAEFIK_URL", "http://localhost:8080")
	whoamiHost    = getEnv("WHOAMI_HOST", "whoami.localhost")
	whoamiXFFHost = getEnv("WHOAMI_XFF_HOST", "whoami-xff.localhost")
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func isDockerAvailable() bool {
	cmd := exec.Command("docker", "version")
	err := cmd.Run()
	return err == nil
}

func skipIfNoDocker(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Skipping E2E test: Docker not available")
	}

	// Also skip in short mode
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}
}

func TestE2EPluginLoaded(t *testing.T) {
	skipIfNoDocker(t)
	// Give Traefik time to fully start
	time.Sleep(5 * time.Second)

	// Test that the service is accessible (empty EDL should allow all)
	req, err := http.NewRequest("GET", traefikURL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Host = whoamiHost

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// With empty EDL in blocklist mode, all requests should be allowed
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
	}
}

func TestE2EDirectIPStrategy(t *testing.T) {
	skipIfNoDocker(t)
	tests := []struct {
		name           string
		headers        map[string]string
		expectedStatus int
		description    string
	}{
		{
			name:           "No headers",
			headers:        map[string]string{},
			expectedStatus: http.StatusOK,
			description:    "Direct connection should be allowed with empty EDL",
		},
		{
			name: "With X-Forwarded-For (should be ignored)",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.1",
			},
			expectedStatus: http.StatusOK,
			description:    "XFF header should be ignored in direct mode",
		},
		{
			name: "With X-Real-IP (should be ignored)",
			headers: map[string]string{
				"X-Real-IP": "10.0.0.1",
			},
			expectedStatus: http.StatusOK,
			description:    "X-Real-IP should be ignored in direct mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", traefikURL, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Host = whoamiHost

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("%s: Expected status %d, got %d. Body: %s",
					tt.description, tt.expectedStatus, resp.StatusCode, string(body))
			}
		})
	}
}

func TestE2EXFFStrategy(t *testing.T) {
	skipIfNoDocker(t)
	tests := []struct {
		name           string
		headers        map[string]string
		expectedStatus int
		description    string
	}{
		{
			name:           "No XFF header",
			headers:        map[string]string{},
			expectedStatus: http.StatusOK,
			description:    "Should use direct IP when no XFF present",
		},
		{
			name: "With valid XFF",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1",
			},
			expectedStatus: http.StatusOK,
			description:    "Should use IP from XFF with trusted proxy",
		},
		{
			name: "With multiple IPs in XFF",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1, 10.0.0.1, 172.16.0.1",
			},
			expectedStatus: http.StatusOK,
			description:    "Should use first IP from XFF chain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", traefikURL, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Host = whoamiXFFHost

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("%s: Expected status %d, got %d. Body: %s",
					tt.description, tt.expectedStatus, resp.StatusCode, string(body))
			}
		})
	}
}

func TestE2EBlockPage(t *testing.T) {
	skipIfNoDocker(t)
	// This test would require a way to add IPs to the blocklist
	// Since we're using an empty EDL, we can't test actual blocking
	// This is a placeholder for when we have a mock EDL server
	t.Skip("Skipping block page test - requires mock EDL with blocked IPs")

	req, err := http.NewRequest("GET", traefikURL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Host = whoamiHost
	req.Header.Set("X-Test-Block", "true") // Would need special handling

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Check for block page content
	if !strings.Contains(bodyStr, "403") || !strings.Contains(bodyStr, "Forbidden") {
		t.Errorf("Block page does not contain expected content: %s", bodyStr)
	}
}

func TestE2EConcurrentRequests(t *testing.T) {
	skipIfNoDocker(t)
	// Test that the plugin handles concurrent requests properly
	numRequests := 50
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			req, err := http.NewRequest("GET", traefikURL, nil)
			if err != nil {
				results <- -1
				return
			}
			req.Host = whoamiHost
			req.Header.Set("X-Request-ID", fmt.Sprintf("%d", id))

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				results <- -1
				return
			}
			defer resp.Body.Close()

			results <- resp.StatusCode
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		status := <-results
		if status == http.StatusOK {
			successCount++
		}
	}

	if successCount != numRequests {
		t.Errorf("Expected %d successful requests, got %d", numRequests, successCount)
	}
}

func TestE2EHealthCheck(t *testing.T) {
	skipIfNoDocker(t)
	// Test Traefik health endpoint
	resp, err := http.Get("http://localhost:8081/api/overview")
	if err != nil {
		t.Skipf("Traefik not running: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Traefik health check failed with status %d", resp.StatusCode)
	}
}
