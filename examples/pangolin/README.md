# Pangolin Integration Example

Configuration files for adding ELLIO EDL protection to an existing Pangolin installation.

## Documentation

For complete Pangolin integration instructions, see:
**[https://docs.ellio.tech/edl-management/integrations/traefik-middleware/pangolin-integration](https://docs.ellio.tech/edl-management/integrations/traefik-middleware/pangolin-integration)**

## Files

- `traefik_config.yml` - Traefik static configuration snippet to add to Pangolin
- `dynamic_config.yml` - Dynamic configuration snippet with ELLIO middleware
- `config.yml` - Pangolin configuration snippet to apply middleware globally

## Quick Start

1. Follow the documentation link above for step-by-step integration instructions
2. Copy the configuration snippets to your Pangolin installation
3. Add your EDL bootstrap token
4. Restart Traefik

**Note:** These are configuration snippets to be added to your existing Pangolin setup, not a standalone example.
