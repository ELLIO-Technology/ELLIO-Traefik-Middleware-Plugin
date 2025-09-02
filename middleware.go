package ELLIO_Traefik_Middleware_Plugin

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/logger"
	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/logs"
	"github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/pkg/singleton"
)

// init is handled by the logger package

// Config holds the plugin configuration
type Config struct {
	BootstrapToken string   `json:"bootstrapToken,omitempty"`
	LogLevel       string   `json:"logLevel,omitempty"`
	MachineID      string   `json:"machineID,omitempty"`      // Optional machine ID override (defaults to random UUID)
	IPStrategy     string   `json:"ipStrategy,omitempty"`     // "direct" (default), "xff", "real-ip", "custom"
	TrustedHeader  string   `json:"trustedHeader,omitempty"`  // Custom header name when ipStrategy is "custom"
	TrustedProxies []string `json:"trustedProxies,omitempty"` // List of trusted proxy IPs or CIDR ranges
}

// CreateConfig creates the default plugin configuration
func CreateConfig() *Config {
	return &Config{}
}

// EllioMiddleware is the main plugin structure
type EllioMiddleware struct {
	next           http.Handler
	name           string
	config         *Config
	trustedProxies []netip.Prefix // Parsed trusted proxy ranges
}

// New creates a new middleware instance
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	logger.Tracef("Creating new middleware instance - name=%s", name)

	// Set log level from config
	logLevel := config.LogLevel
	if logLevel == "" {
		logLevel = "info" // Default to info level
	}

	level, err := logger.ParseLevel(logLevel)
	if err != nil {
		logger.Warnf("Invalid log level '%s', defaulting to info: %v", logLevel, err)
		level = logger.InfoLevel
	}
	logger.SetLevel(level)

	// Initialize singleton manager on first middleware creation
	logger.Trace("Calling singleton.Initialize...")
	if err := singleton.Initialize(config.BootstrapToken, config.MachineID, config.IPStrategy, config.TrustedHeader, config.TrustedProxies); err != nil {
		logger.Errorf("singleton.Initialize failed: %v", err)
		return nil, err
	}
	logger.Trace("singleton.Initialize succeeded")

	// Parse trusted proxies
	var trustedProxies []netip.Prefix
	if len(config.TrustedProxies) > 0 {
		trustedProxies = parseTrustedProxies(config.TrustedProxies)
		logger.Infof("Parsed %d trusted proxy ranges", len(trustedProxies))
	}

	// Set default IP strategy if not specified
	if config.IPStrategy == "" {
		config.IPStrategy = "direct"
	}

	middleware := &EllioMiddleware{
		next:           next,
		name:           name,
		config:         config,
		trustedProxies: trustedProxies,
	}

	logger.Infof("ELLIO middleware ready: %s", name)
	return middleware, nil
}

// ServeHTTP handles incoming requests
func (e *EllioMiddleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var start time.Time
	var timings map[string]time.Duration
	var debugMode bool

	if logger.IsDebugEnabled() {
		debugMode = true
		start = time.Now()
		timings = make(map[string]time.Duration)
		defer func() {
			totalDuration := time.Since(start)

			// Calculate middleware overhead (total - handler time)
			handlerTime := timings["handler"]
			middlewareOverhead := totalDuration - handlerTime

			// Build middleware timing breakdown
			var middlewareBreakdown string
			for _, key := range []string{"manager", "deploy_check", "ip_extract", "ip_check"} {
				if duration, ok := timings[key]; ok {
					if middlewareBreakdown != "" {
						middlewareBreakdown += ", "
					}
					middlewareBreakdown += fmt.Sprintf("%s=%v", key, duration)
				}
			}

			// Log with clear separation of middleware overhead vs handler time
			if middlewareBreakdown != "" {
				logger.Debugf("REQUEST %s %s - middleware_overhead=%v [%s] handler=%v total=%v",
					req.Method, req.URL.Path, middlewareOverhead, middlewareBreakdown, handlerTime, totalDuration)
			} else {
				// No middleware checks performed (e.g., manager not ready)
				logger.Debugf("REQUEST %s %s - handler=%v total=%v",
					req.Method, req.URL.Path, handlerTime, totalDuration)
			}
		}()
	}

	// Recover from any panics to prevent bad gateway
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Recovered from panic in ServeHTTP: %v", r)
			// Try to return 500 if response not written yet
			http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		}
	}()

	// Get singleton manager instance
	var managerStart time.Time
	if debugMode {
		managerStart = time.Now()
	}
	manager := singleton.GetManager()
	if debugMode {
		timings["manager"] = time.Since(managerStart)
	}

	// If manager is not ready or deployment is disabled, allow all traffic
	if manager == nil {
		if debugMode {
			handlerStart := time.Now()
			e.next.ServeHTTP(rw, req)
			timings["handler"] = time.Since(handlerStart)
		} else {
			e.next.ServeHTTP(rw, req)
		}
		return
	}

	var deployStart time.Time
	if debugMode {
		deployStart = time.Now()
	}
	deploymentEnabled := manager.IsDeploymentEnabled()
	if debugMode {
		timings["deploy_check"] = time.Since(deployStart)
	}

	if !deploymentEnabled {
		if debugMode {
			handlerStart := time.Now()
			e.next.ServeHTTP(rw, req)
			timings["handler"] = time.Since(handlerStart)
		} else {
			e.next.ServeHTTP(rw, req)
		}
		return
	}

	// Extract client IP
	var ipExtractStart time.Time
	if debugMode {
		ipExtractStart = time.Now()
	}
	clientIP := e.extractClientIP(req)
	if debugMode {
		timings["ip_extract"] = time.Since(ipExtractStart)
	}
	logger.Tracef("Extracted client IP: %s", clientIP)

	if clientIP == "" {
		logger.Debug("Empty client IP, returning 400")
		http.Error(rw, "Unable to determine client IP", http.StatusBadRequest)
		return
	}

	// Check if IP is allowed based on EDL
	var allowed bool
	var err error
	if debugMode {
		ipCheckStart := time.Now()
		allowed, _, err = manager.IsIPAllowedWithStats(clientIP)
		checkDuration := time.Since(ipCheckStart)
		timings["ip_check"] = checkDuration
	} else {
		allowed, err = manager.IsIPAllowed(clientIP)
	}
	if err != nil {
		logger.Debugf("IP validation error, returning 400: %v", err)
		http.Error(rw, "Invalid IP address", http.StatusBadRequest)
		return
	}

	if allowed {
		// Fast path for allowed requests - no event creation
		if debugMode {
			handlerStart := time.Now()
			e.next.ServeHTTP(rw, req)
			timings["handler"] = time.Since(handlerStart)
		} else {
			e.next.ServeHTTP(rw, req)
		}
		return
	}

	logger.Debug("Request BLOCKED, returning 403")
	ServeBlockPage(rw)

	// Create and send event for blocked request
	logger.Trace("Preparing log event for blocked request...")

	scheme := "http"
	if req.TLS != nil || req.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}

	// Get direct IP for debugging
	directIP := getDirectIP(req.RemoteAddr)

	logger.Tracef("Creating block event - method=%s host=%s path=%s extractedIP=%s directIP=%s",
		req.Method, req.Host, req.URL.Path, clientIP, directIP)

	event := logs.NewBlockEvent(
		clientIP, // extracted IP that was checked
		directIP, // direct connection IP
		req.Method,
		req.Host,
		req.URL.Path,
		scheme,
		req.Header.Get("User-Agent"),
		manager.GetEDLMode(),
	)

	logger.Trace("Sending blocked event to log shipper")
	manager.SendBlockEvent(event)
	logger.Trace("ServeHTTP completed for blocked request")
}

func (e *EllioMiddleware) extractClientIP(r *http.Request) string {
	// Extract the direct connection IP
	directIP := getDirectIP(r.RemoteAddr)

	// If strategy is direct or no trusted proxies configured, return direct IP
	if e.config.IPStrategy == "direct" || len(e.trustedProxies) == 0 {
		return directIP
	}

	// Check if request is from a trusted proxy
	if !e.isFromTrustedProxy(directIP) {
		logger.Warnf("Request from untrusted proxy %s, ignoring headers", directIP)
		return directIP
	}

	// Extract IP based on configured strategy
	switch e.config.IPStrategy {
	case "xff":
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// X-Forwarded-For can contain multiple IPs, take the first one
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
	case "real-ip":
		if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			return strings.TrimSpace(realIP)
		}
	case "custom":
		if e.config.TrustedHeader != "" {
			if customIP := r.Header.Get(e.config.TrustedHeader); customIP != "" {
				return strings.TrimSpace(customIP)
			}
		}
	}

	// Fall back to direct IP if header extraction failed
	return directIP
}

func getDirectIP(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

func (e *EllioMiddleware) isFromTrustedProxy(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}

	for _, trusted := range e.trustedProxies {
		if trusted.Contains(addr) {
			return true
		}
	}

	return false
}

func parseTrustedProxies(proxies []string) []netip.Prefix {
	var result []netip.Prefix

	for _, proxy := range proxies {
		// Handle special keywords
		switch strings.ToLower(proxy) {
		case "loopback":
			// Add loopback ranges
			if prefix, err := netip.ParsePrefix("127.0.0.0/8"); err == nil {
				result = append(result, prefix)
			}
			if prefix, err := netip.ParsePrefix("::1/128"); err == nil {
				result = append(result, prefix)
			}
			continue
		case "private":
			// Add private network ranges (RFC 1918)
			privateRanges := []string{
				"10.0.0.0/8",
				"172.16.0.0/12",
				"192.168.0.0/16",
				"fc00::/7",  // IPv6 unique local
				"fe80::/10", // IPv6 link-local
			}
			for _, r := range privateRanges {
				if prefix, err := netip.ParsePrefix(r); err == nil {
					result = append(result, prefix)
				}
			}
			continue
		}

		// Try to parse as CIDR
		if prefix, err := netip.ParsePrefix(proxy); err == nil {
			result = append(result, prefix)
			continue
		}

		// Try to parse as single IP and convert to /32 or /128
		if addr, err := netip.ParseAddr(proxy); err == nil {
			if addr.Is4() {
				if prefix, err := netip.ParsePrefix(proxy + "/32"); err == nil {
					result = append(result, prefix)
				}
			} else if addr.Is6() {
				if prefix, err := netip.ParsePrefix(proxy + "/128"); err == nil {
					result = append(result, prefix)
				}
			}
			continue
		}

		logger.Warnf("Failed to parse trusted proxy: %s", proxy)
	}

	return result
}
