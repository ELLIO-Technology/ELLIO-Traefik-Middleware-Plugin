# Adding ELLIO EDL Protection to Pangolin

This example demonstrates how to add ELLIO Traefik Middleware Plugin to an existing [Pangolin](https://github.com/fosrl/pangolin) installation for IP-based access control.

## Overview

This guide assumes you have Pangolin already installed and running. We'll add the ELLIO EDL (External Dynamic List) middleware to automatically protect all Pangolin services with IP filtering.

## What This Adds

The ELLIO middleware will:
- Filter all incoming traffic by IP address
- Use your EDL rules from the ELLIO platform
- Automatically apply to all Pangolin-managed services
- Provide centralized IP management through ELLIO platform

## Prerequisites

- Existing Pangolin installation
- ELLIO Platform account with an EDL configured for Traefik Middleware
- Access to modify Pangolin configuration files

## Installation Steps

### 1. Add ELLIO Plugin to Traefik

Edit your existing `config/traefik/traefik.yml` and add the ELLIO plugin:

```yaml
experimental:
  plugins:
    # Your existing plugins...
    ellio:
      moduleName: "github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin"
      version: "v1.0.0"
```

### 2. Configure ELLIO Middleware

Add the following to your `config/traefik/dynamic_config.yml`:

```yaml
http:
  middlewares:
    # Your existing middlewares...

    ellio-edl:
      plugin:
        ellio:
          bootstrapToken: "your-edl-bootstrap-token-here"
          logLevel: "info"
          ipStrategy: "direct"  # or "xff" if behind proxy
          machineId: "pangolin-traefik"  # Optional identifier
```

Replace `"your-edl-bootstrap-token-here"` with your actual EDL bootstrap token from the ELLIO platform.

### 3. Apply Middleware to Pangolin Services

Edit your `config/config.yml` and add the ELLIO middleware to Pangolin's additional middlewares:

```yaml
traefik:
    additional_middlewares: ["ellio-edl@file"]
```

This tells Pangolin to automatically apply the ELLIO EDL middleware to all protected services.

### 4. Restart Services

Restart Traefik to load the new configuration:

```bash
docker compose restart traefik
```

## Configuration Options

### IP Strategy

Configure how the middleware extracts client IPs:

- `direct` - Use the direct connection IP (default)
- `xff` - Extract from X-Forwarded-For header
- `real-ip` - Extract from X-Real-IP header
- `custom` - Use a custom header

### Behind CloudFlare

If your Pangolin instance is behind CloudFlare, use the custom header:

```yaml
ellio-edl:
  plugin:
    ellio:
      bootstrapToken: "your-edl-bootstrap-token-here"
      ipStrategy: "custom"
      trustedHeader: "CF-Connecting-IP"
      trustedProxies:
        - "173.245.48.0/20"  # CloudFlare IP range
        - "103.21.244.0/22"  # CloudFlare IP range
        - "103.22.200.0/22"  # Add all CloudFlare ranges
```

### Behind Other Proxies

For generic reverse proxies using X-Forwarded-For:

```yaml
ellio-edl:
  plugin:
    ellio:
      bootstrapToken: "your-edl-bootstrap-token-here"
      ipStrategy: "xff"
      trustedProxies:
        - "10.0.0.0/8"      # Your proxy IP range
        - "192.168.0.0/16"  # Private network ranges
```

### Machine ID

Optionally set a machine ID to identify this instance in ELLIO logs:

```yaml
machineId: "production-pangolin"  # or "staging", etc.
```

## Verification

### Check Middleware is Active

After restarting, verify the middleware is loaded:

```bash
# Check Traefik logs
docker compose logs traefik | grep -i ellio

# You should see initialization messages
```

### Test IP Filtering

For EDL in **blocklist mode** (most common):

1. Add an IP to block in the ELLIO platform:
   - Navigate to your IP Ruleset that's included in the EDL
   - Add a test IP address to block
   - Wait for EDL regeneration (check your EDL's update frequency)
2. Test from the blocked IP - should receive 403 Forbidden
3. Test from a non-blocked IP - should access normally

For EDL in **allowlist mode**:

1. Add your IP to the IP Ruleset in ELLIO platform
2. Wait for EDL regeneration based on configured frequency
3. Try accessing Pangolin dashboard - should work
4. Test from a non-allowed IP - should be blocked

## File Structure

After adding ELLIO middleware, your config structure should include:

```
config/
├── config.yml                    # Modified: Added ellio-edl to additional_middlewares
└── traefik/
    ├── traefik.yml               # Modified: Added ELLIO plugin
    └── dynamic_config.yml        # Modified: Added ellio-edl middleware definition
```

## Troubleshooting

### Middleware Not Loading

- Check bootstrap token is valid and properly formatted
- Verify plugin version in `traefik.yml`
- Check Traefik logs for initialization errors

### All IPs Blocked

- Verify EDL mode (blocklist vs allowlist) in ELLIO platform
- Check if EDL is active and not deleted
- Confirm your IP is in the appropriate list

### IPs Not Being Filtered

- Ensure `additional_middlewares` includes `ellio-edl@file`
- Verify middleware is applied to routes in Traefik dashboard
- Check `ipStrategy` matches your network setup

## Support

- **Pangolin Documentation**: [https://docs.digpangolin.com](https://docs.digpangolin.com)
- **ELLIO Platform**: [https://platform.ellio.tech](https://platform.ellio.tech)
- **Issues**: [GitHub Issues](https://github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/issues)
