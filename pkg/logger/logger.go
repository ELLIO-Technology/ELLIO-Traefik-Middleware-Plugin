package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// LogLevel represents the logging level
type LogLevel int

const (
	TraceLevel LogLevel = iota
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
)

// Use atomic for lock-free log level access
var currentLevel atomic.Int32

func init() {
	// Ensure output goes to stdout for Traefik
	log.SetOutput(os.Stdout)
	// Remove timestamp as Traefik adds its own
	log.SetFlags(0)
	// Initialize default log level
	currentLevel.Store(int32(InfoLevel)) //nolint:gosec // LogLevel values are small constants (0-4)
}

// SetLevel sets the global log level
func SetLevel(level LogLevel) {
	currentLevel.Store(int32(level)) //nolint:G115 // LogLevel values are small constants (0-4)
}

// ParseLevel parses a string log level
func ParseLevel(level string) (LogLevel, error) {
	switch strings.ToLower(level) {
	case "trace":
		return TraceLevel, nil
	case "debug":
		return DebugLevel, nil
	case "info":
		return InfoLevel, nil
	case "warn", "warning":
		return WarnLevel, nil
	case "error":
		return ErrorLevel, nil
	default:
		return InfoLevel, fmt.Errorf("invalid log level: %s", level)
	}
}

// shouldLog checks if a message at the given level should be logged
func shouldLog(level LogLevel) bool {
	return level >= LogLevel(currentLevel.Load())
}

// IsTraceEnabled returns true if trace logging is enabled
func IsTraceEnabled() bool {
	return LogLevel(currentLevel.Load()) <= TraceLevel
}

// IsDebugEnabled returns true if debug logging is enabled
func IsDebugEnabled() bool {
	return LogLevel(currentLevel.Load()) <= DebugLevel
}

// getTimestamp returns the current UTC timestamp in RFC3339 format
func getTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// Trace logs a trace message
func Trace(args ...interface{}) {
	if shouldLog(TraceLevel) {
		log.Print(getTimestamp(), " [TRACE] ", fmt.Sprint(args...))
	}
}

// Tracef logs a formatted trace message
func Tracef(format string, args ...interface{}) {
	if shouldLog(TraceLevel) {
		log.Printf("%s [TRACE] "+format, append([]interface{}{getTimestamp()}, args...)...)
	}
}

// Debug logs a debug message
func Debug(args ...interface{}) {
	if shouldLog(DebugLevel) {
		log.Print(getTimestamp(), " [DEBUG] ", fmt.Sprint(args...))
	}
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...interface{}) {
	if shouldLog(DebugLevel) {
		log.Printf("%s [DEBUG] "+format, append([]interface{}{getTimestamp()}, args...)...)
	}
}

// Info logs an info message
func Info(args ...interface{}) {
	if shouldLog(InfoLevel) {
		log.Print(getTimestamp(), " [INFO] ", fmt.Sprint(args...))
	}
}

// Infof logs a formatted info message
func Infof(format string, args ...interface{}) {
	if shouldLog(InfoLevel) {
		log.Printf("%s [INFO] "+format, append([]interface{}{getTimestamp()}, args...)...)
	}
}

// Warn logs a warning message
func Warn(args ...interface{}) {
	if shouldLog(WarnLevel) {
		log.Print(getTimestamp(), " [WARN] ", fmt.Sprint(args...))
	}
}

// Warnf logs a formatted warning message
func Warnf(format string, args ...interface{}) {
	if shouldLog(WarnLevel) {
		log.Printf("%s [WARN] "+format, append([]interface{}{getTimestamp()}, args...)...)
	}
}

// Error logs an error message
func Error(args ...interface{}) {
	if shouldLog(ErrorLevel) {
		log.Print(getTimestamp(), " [ERROR] ", fmt.Sprint(args...))
	}
}

// Errorf logs a formatted error message
func Errorf(format string, args ...interface{}) {
	if shouldLog(ErrorLevel) {
		log.Printf("%s [ERROR] "+format, append([]interface{}{getTimestamp()}, args...)...)
	}
}

// WithField is a simple helper that formats a field into the message
func WithField(key string, value interface{}) string {
	return fmt.Sprintf("%s=%v", key, value)
}

// WithError formats an error into the message
func WithError(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("error=%v", err)
}
