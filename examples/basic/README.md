# Basic ELLIO Traefik Middleware Example

Production-ready setup using the ELLIO Traefik Middleware Plugin from the official plugin catalog.

## Documentation

For complete setup and configuration instructions, see:
**[https://docs.ellio.tech/edl-management/integrations/traefik-middleware/simple-setup](https://docs.ellio.tech/edl-management/integrations/traefik-middleware/simple-setup)**

## Files

- `docker-compose.yml` - Container orchestration
- `traefik-static.yml` - Traefik static configuration with plugin
- `traefik-dynamic.yml` - Middleware and routing rules (add your bootstrap token here)

## Quick Start

1. Edit `traefik-dynamic.yml` and add your EDL bootstrap token
2. Run `docker compose up -d`

See the documentation link above for detailed configuration options and troubleshooting.
