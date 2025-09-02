#!/bin/bash
# Setup script for local development
# This script generates traefik-dynamic.yml from template using your .env file

# Load .env file if it exists
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
    echo "Loaded configuration from .env"
else
    echo "Warning: .env file not found. Copy .env.example to .env and add your token."
    echo "Using placeholder token for now..."
    export BOOTSTRAP_TOKEN="invalid-jwt-placeholder"
fi

# Generate traefik-dynamic.yml from template
envsubst < traefik-dynamic.yml.template > traefik-dynamic.yml
echo "Generated traefik-dynamic.yml with bootstrap token"

echo "Ready to run: docker compose -f docker-compose.test.yml up"
