# Development Guide

This guide covers the development setup, testing procedures, and contribution guidelines for the ELLIO Traefik Middleware Plugin.

## Prerequisites

- Go 1.21 or later
- Docker and Docker Compose
- Make (for convenience commands)
- golangci-lint (for code quality checks)
- Yaegi (for Traefik plugin compatibility testing)

## Development Setup

### 1. Clone the Repository

```bash
git clone https://github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin.git
cd ELLIO-Traefik-Middleware-Plugin
```

### 2. Install Dependencies

```bash
# Download Go dependencies
go mod download

# Create vendor directory (required for Traefik plugins)
go mod vendor

# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/traefik/yaegi/cmd/yaegi@latest
```

### 3. Environment Configuration

```bash
# Copy the example environment file
cp .env.example .env

# Edit .env and add your EDL bootstrap token
# BOOTSTRAP_TOKEN=your_edl_bootstrap_token_here
```

### 4. Generate Test Configuration

```bash
# Generate Traefik dynamic configuration from template
envsubst < ci/traefik-dynamic.yml.template > ci/traefik-dynamic.yml
```

## Project Structure

```
.
├── .github/workflows/     # CI/CD workflows
├── ci/                     # CI/CD and testing infrastructure
│   ├── e2e/               # End-to-end tests
│   ├── docker-compose.test.yml
│   ├── Dockerfile.test
│   └── traefik-dynamic.yml.template
├── pkg/                    # Internal packages
│   ├── api/               # API client for ELLIO platform
│   ├── ipmatcher/         # IP matching logic
│   ├── iptrie/            # Trie data structure for IPs
│   ├── logger/            # Logging utilities
│   ├── logs/              # Event logging
│   ├── singleton/         # Singleton manager
│   └── utils/             # Utility functions
├── vendor/                # Vendored dependencies
├── .traefik.yml          # Traefik plugin metadata
├── go.mod                # Go module definition
├── middleware.go         # Main middleware implementation
├── middleware_test.go    # Middleware tests
└── Makefile             # Development commands
```

## Testing

### Running Tests

```bash
# Run all tests with coverage
make test

# Run unit tests only (fast)
make test-unit

# Run integration tests
make test-integration

# Run end-to-end tests with Docker
make test-e2e

# Run benchmarks
make bench

# Generate HTML coverage report
make coverage
```

### Test Categories

#### Unit Tests

Fast, isolated tests for individual components:

```bash
go test -v -short ./...
```

#### Integration Tests

Tests that verify component interactions:

```bash
go test -v -run Integration ./...
```

#### End-to-End Tests

Full stack tests using Docker Compose:

```bash
# Start test environment
docker compose -f ci/docker-compose.test.yml up -d

# Run E2E tests
cd ci/e2e && go test -v -tags=e2e ./...

# View logs
docker compose -f ci/docker-compose.test.yml logs traefik

# Clean up
docker compose -f ci/docker-compose.test.yml down -v
```

### Writing Tests

#### Unit Test Example

```go
func TestIPExtraction(t *testing.T) {
    tests := []struct {
        name       string
        remoteAddr string
        headers    map[string]string
        expected   string
    }{
        {
            name:       "direct IP",
            remoteAddr: "192.168.1.1:8080",
            expected:   "192.168.1.1",
        },
        // Add more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

#### E2E Test Example

```go
//go:build e2e
// +build e2e

func TestE2EPluginBlocking(t *testing.T) {
    skipIfNoDocker(t)

    // Make request to Traefik
    resp, err := http.Get("http://localhost:8080")
    require.NoError(t, err)
    defer resp.Body.Close()

    // Verify response
    assert.Equal(t, http.StatusOK, resp.StatusCode)
}
```

## Code Quality

### Linting

```bash
# Run all linters
make lint

# Run golangci-lint directly
golangci-lint run ./...

# Auto-fix issues where possible
golangci-lint run --fix ./...
```

### Formatting

```bash
# Format all Go code
make fmt

# Check if code is formatted
make fmt-check
```

### Security Scanning

```bash
# Run gosec security scanner
make security

# Run Trivy vulnerability scanner (via CI)
docker run --rm -v $(pwd):/src aquasec/trivy fs /src
```

### Yaegi Compatibility

Traefik uses Yaegi interpreter for plugins. Test compatibility:

```bash
# Test plugin with Yaegi
make yaegi-test

# Manual Yaegi testing
yaegi test -v .
```

## Local Development with Traefik

### Start Local Environment

```bash
# Start Traefik with the plugin
make docker-up

# View logs
make docker-logs

# Stop environment
make docker-down
```

### Testing with curl

```bash
# Test direct access
curl -H "Host: whoami.localhost" http://localhost:8080

# Test with X-Forwarded-For
curl -H "Host: whoami-xff.localhost" \
     -H "X-Forwarded-For: 192.168.1.100" \
     http://localhost:8080

# Test with custom header
curl -H "Host: whoami.localhost" \
     -H "CF-Connecting-IP: 203.0.113.1" \
     http://localhost:8080
```

## Debugging

### Enable Debug Logging

```yaml
# In your middleware configuration
ellio:
  bootstrapToken: "${ELLIO_BOOTSTRAP}"
  logLevel: "debug"  # or "trace" for maximum verbosity
```

### Common Issues

#### Plugin Not Loading

1. Check `.traefik.yml` exists and is valid
2. Verify `go.mod` module name matches plugin configuration
3. Ensure vendor directory is up to date: `go mod vendor`

#### Token Issues

1. Verify bootstrap token is valid
2. Check token has correct component_type: `ellio_traefik_middleware_plugin`
3. Ensure EDL exists and is active in ELLIO platform

#### IP Extraction Issues

1. Enable debug logging to see extracted IPs
2. Verify trusted proxy configuration
3. Check header names match your infrastructure

## CI/CD

### GitHub Actions Workflows

#### CI Workflow (`.github/workflows/ci.yml`)

Runs on every push and pull request:

- Linting and formatting checks
- Unit and integration tests
- Security scanning (gosec, trivy)
- Yaegi compatibility testing
- E2E tests (requires BOOTSTRAP_TOKEN secret)
- Benchmark tests

#### Release Workflow (`.github/workflows/release.yml`)

Triggered by version tags (v*):

- Validates semver format
- Runs full test suite
- Creates GitHub release
- Generates changelog
- Creates release archives

### Creating a Release

```bash
# Tag a new version
git tag v1.0.0
git push origin v1.0.0

# The release workflow will automatically:
# - Run all tests
# - Create GitHub release
# - Generate changelog
# - Create release archives
```

## Git Hooks

### Installing Pre-commit Hooks

```bash
# Using pre-commit framework
make install-hooks

# Or use custom git hooks
git config core.hooksPath .githooks
```

### Manual Hook Run

```bash
# Run hooks on all files
make run-hooks
```

## Benchmarking

### Running Benchmarks

```bash
# Run all benchmarks
make bench

# Run specific benchmark
go test -bench=BenchmarkIPMatching -benchmem ./pkg/ipmatcher

# Run with CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./...
go tool pprof cpu.prof
```

### Writing Benchmarks

```go
func BenchmarkIPLookup(b *testing.B) {
    matcher := NewMatcher()
    matcher.Update([]string{"192.168.0.0/16", "10.0.0.0/8"})

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        matcher.Contains("192.168.1.1")
    }
}
```

## Contributing

### Development Workflow

1. Fork the repository
2. Create a feature branch
   ```bash
   git checkout -b feature/your-feature-name
   ```
3. Make your changes
4. Add/update tests
5. Run tests and linting
   ```bash
   make ci
   ```
6. Commit with descriptive message
   ```bash
   git commit -m "feat: add support for X feature"
   ```
7. Push to your fork
   ```bash
   git push origin feature/your-feature-name
   ```
8. Create a Pull Request

### Commit Message Format

Follow conventional commits format:

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `test:` Test additions or changes
- `refactor:` Code refactoring
- `perf:` Performance improvements
- `chore:` Maintenance tasks

### Code Review Checklist

- [ ] Tests pass (`make test`)
- [ ] Code is formatted (`make fmt`)
- [ ] Linting passes (`make lint`)
- [ ] Documentation updated if needed
- [ ] Yaegi compatibility verified (`make yaegi-test`)
- [ ] No security issues (`make security`)

## Makefile Commands

```bash
make help              # Show all available commands

# Development
make test             # Run all tests
make test-unit        # Run unit tests
make test-e2e         # Run E2E tests
make bench            # Run benchmarks
make coverage         # Generate coverage report

# Code Quality
make lint             # Run linters
make fmt              # Format code
make security         # Security scan
make yaegi-test       # Yaegi compatibility

# Docker
make docker-up        # Start test environment
make docker-down      # Stop test environment
make docker-logs      # View Traefik logs

# CI/CD
make ci               # Run all CI checks
make pre-commit       # Run pre-commit checks

# Dependencies
make deps             # Download dependencies
make vendor           # Update vendor directory
make clean            # Clean build artifacts
```

## Resources

- [Traefik Plugin Documentation](https://doc.traefik.io/traefik/plugins/)
- [Yaegi Interpreter](https://github.com/traefik/yaegi)
- [Go Testing Documentation](https://golang.org/pkg/testing/)
- [ELLIO Platform Documentation](https://docs.ellio.tech)

## Support

For development questions and issues:

- Create an issue on [GitHub](https://github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/issues)
- Check existing issues for solutions
- Include logs and configuration when reporting issues
