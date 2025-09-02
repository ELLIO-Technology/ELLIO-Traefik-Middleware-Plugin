# Local Plugin Development Example

This example demonstrates how to run the ELLIO Traefik Middleware Plugin locally for development and testing.

## Prerequisites

- Docker and Docker Compose
- An EDL (External Dynamic List) configured in the ELLIO platform
- Bootstrap token from your EDL

## Setup

1. **Edit the configuration file:**
   Open `traefik-dynamic.yml` and replace `"CHANGEME"` with your actual EDL bootstrap token:
   ```yaml
   bootstrapToken: "your-actual-edl-bootstrap-token"
   ```

2. **Start the services:**
   ```bash
   docker compose up
   ```

## Testing

The setup includes two routes with different IP extraction strategies:

### Direct IP Strategy
Test the direct IP extraction:
```bash
curl -H "Host: whoami.localhost" http://localhost:8080
```

### X-Forwarded-For Strategy
Test with proxy headers:
```bash
curl -H "Host: whoami-xff.localhost" \
     -H "X-Forwarded-For: 192.168.1.100" \
     http://localhost:8080
```

## Services

- **Traefik Dashboard**: http://localhost:8081
- **Whoami (direct)**: http://whoami.localhost:8080
- **Whoami (XFF)**: http://whoami-xff.localhost:8080

## Configuration Files

- `docker-compose.yml` - Container orchestration
- `traefik-static.yml` - Traefik static configuration with local plugin
- `traefik-dynamic.yml` - Middleware and routing configuration (edit this to add your token)

## Development Workflow

1. Make changes to the plugin code in the main repository
2. The plugin is automatically reloaded by Traefik
3. Test your changes using the curl commands above
4. Check logs: `docker compose logs -f traefik`

## Debugging

To enable debug logging, edit `traefik-static.yml`:
```yaml
log:
  level: DEBUG
```

Or modify the middleware configuration in `traefik-dynamic.yml`:
```yaml
ellio-direct:
  plugin:
    ellio:
      logLevel: "debug"
```

## Cleanup

```bash
docker compose down
```
