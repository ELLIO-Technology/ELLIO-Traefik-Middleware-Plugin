<div align="center">

![ELLIO Traefik Middleware Plugin Banner](./assets/banner.png)

# ELLIO Traefik Middleware Plugin

**Dynamic IP-based access control for Traefik using ELLIO External Dynamic Lists (EDL)**

[![Technical Preview](https://img.shields.io/badge/Status-Technical%20Preview-yellow?style=flat)](https://github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![CI Status](https://img.shields.io/github/actions/workflow/status/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/ci.yml?branch=main&style=flat&label=CI)](https://github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/actions)
[![GitHub Release](https://img.shields.io/github/v/release/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin?style=flat)](https://github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/releases)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue?style=flat)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin)](https://goreportcard.com/report/github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin)

</div>

---

## Overview

The ELLIO Traefik Middleware Plugin provides IP-based access control for services behind Traefik proxy. It integrates with the ELLIO platform to dynamically manage IP allowlists and blocklists through External Dynamic Lists (EDL).

## Prerequisites

- Traefik v3.0 or later
- ELLIO Platform account
- An External Dynamic List (EDL) configured for Traefik Middleware
- Bootstrap token from your EDL configuration

## Documentation

Complete documentation is available at: **[https://docs.ellio.tech/edl-management/integrations/traefik-middleware/](https://docs.ellio.tech/edl-management/integrations/traefik-middleware/)**

### Setup Guides

- **[Simple Setup](https://docs.ellio.tech/edl-management/integrations/traefik-middleware/simple-setup)** - Standard Traefik deployment
- **[Pangolin Integration](https://docs.ellio.tech/edl-management/integrations/traefik-middleware/pangolin-integration)** - Add ELLIO protection to Pangolin
- **[CloudFlare Setup](https://docs.ellio.tech/edl-management/integrations/traefik-middleware/cloudflare-setup)** - Configuration for CloudFlare proxy

## Examples

Ready-to-run examples are available in the `examples/` directory:

- **[examples/basic/](examples/basic/)** - Production setup with plugin catalog
- **[examples/local-plugin/](examples/local-plugin/)** - Local development setup
- **[examples/pangolin/](examples/pangolin/)** - Pangolin integration example

## Development

For development setup, testing, and contribution guidelines, see [DEVELOPMENT.md](DEVELOPMENT.md).

## Support

- **Documentation**: [https://docs.ellio.tech](https://docs.ellio.tech/edl-management/integrations/traefik-middleware/)
- **Issues**: [GitHub Issues](https://github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin/issues)
- **ELLIO Platform**: [https://platform.ellio.tech](https://platform.ellio.tech)

## License

Copyright © 2025 ELLIO Technology s.r.o.

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.

## Trademarks

- ELLIO is a registered trademark of ELLIO Technology s.r.o.
- Traefik® is a registered trademark of Traefik Labs
- All other trademarks are property of their respective owners.

---

<div align="center">
  Part of the <a href="https://platform.ellio.tech">ELLIO EDL Management Platform</a>
  <br>
  Copyright © ELLIO Technology s.r.o.
</div>
