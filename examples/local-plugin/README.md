# Local Plugin Development Example

Local development setup for testing the ELLIO Traefik Middleware Plugin.

## Documentation

For complete documentation, see:
**[https://docs.ellio.tech/edl-management/integrations/traefik-middleware/](https://docs.ellio.tech/edl-management/integrations/traefik-middleware/)**

## Files

- `docker-compose.yml` - Container orchestration with local plugin mount
- `traefik-static.yml` - Traefik configuration for local plugin development
- `traefik-dynamic.yml` - Middleware and routing configuration (add your bootstrap token here)

## Quick Start

1. Edit `traefik-dynamic.yml` and add your EDL bootstrap token
2. Run `docker compose up`

**Note:** This setup is for development only. For production, use the plugin catalog version as shown in the basic example.
