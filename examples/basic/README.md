# Basic ELLIO Traefik Middleware Example

This example demonstrates a production-ready setup using the ELLIO Traefik Middleware Plugin from the official plugin catalog.

## Quick Start

### 1. Prerequisites

- Docker and Docker Compose
- An ELLIO Platform account
- An External Dynamic List (EDL) configured for Traefik Middleware

### 2. Create an EDL

1. Log into the [ELLIO Platform](https://platform.ellio.tech)
2. Navigate to EDLs section
3. Create a new EDL with type "Traefik Middleware"
4. Configure your IP rules (allowlist/blocklist)
5. Copy the bootstrap token

### 3. Configure

Edit `traefik-dynamic.yml` and replace `"CHANGEME"` with your EDL bootstrap token:

```yaml
ellio-edl:
  plugin:
    ellio:
      bootstrapToken: "your-actual-edl-bootstrap-token"
```

### 4. Deploy

```bash
docker compose up -d
```

## Testing

Test the protected service:
```bash
# Replace app.example.com with your actual domain
curl -H "Host: app.example.com" http://localhost

# Or add to /etc/hosts for local testing
echo "127.0.0.1 app.example.com" | sudo tee -a /etc/hosts
curl http://app.example.com
```

## Configuration

### IP Strategy

The example uses `xff` strategy for environments behind proxies. Adjust in `traefik-dynamic.yml`:

- `direct` - Use for direct internet-facing deployments
- `xff` - Use when behind a reverse proxy or CDN
- `real-ip` - Use with proxies that set X-Real-IP
- `custom` - Use with custom header (requires `trustedHeader`)

### Trusted Proxies

Update the `trustedProxies` list based on your infrastructure:

```yaml
trustedProxies:
  - "10.0.0.0/8"      # Private networks
  - "172.16.0.0/12"   # Private networks
  - "192.168.0.0/16"  # Private networks
  # Add your proxy/CDN ranges here
```

## Production Considerations

1. **Dashboard Security**: Remove or secure the Traefik dashboard
2. **HTTPS**: Configure proper certificates for websecure entrypoint
3. **Logging**: Adjust log levels and configure log rotation
4. **Monitoring**: Integrate with your monitoring solution

## Services

- **Traefik Dashboard**: http://localhost:8080 (disable in production)
- **Protected App**: http://app.example.com

## Files

- `docker-compose.yml` - Container orchestration
- `traefik-static.yml` - Traefik static configuration
- `traefik-dynamic.yml` - Middleware and routing rules (edit this to add your token)

## Troubleshooting

Check logs:
```bash
docker compose logs -f traefik
```

Verify EDL is active in ELLIO platform:
- Log into platform
- Check EDL status
- Verify IP rules

## Cleanup

```bash
docker compose down
```
