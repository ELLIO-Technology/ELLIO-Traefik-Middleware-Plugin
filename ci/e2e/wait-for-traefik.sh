#!/bin/sh

set -e

echo "Waiting for Traefik to be ready..."

# Wait for Traefik API to be available
until curl -f http://traefik:8080/api/overview > /dev/null 2>&1; do
    echo "Waiting for Traefik API..."
    sleep 2
done

echo "Traefik API is ready"

# Wait for whoami service to be registered
until curl -f -H "Host: whoami.localhost" http://traefik:80 > /dev/null 2>&1; do
    echo "Waiting for whoami service..."
    sleep 2
done

echo "Services are ready, running E2E tests..."

# Run the E2E tests
cd /tests && go test -v ./...
