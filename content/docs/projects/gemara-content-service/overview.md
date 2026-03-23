---
title: "Overview"
description: "A content delivery and enrichment service for Gemara compliance artifacts."
date: 2026-03-23T08:53:52Z
lastmod: 2026-03-23T08:53:52Z
draft: false
toc: true
weight: 1
params:
  editURL: "https://github.com/complytime/gemara-content-service/edit/main/README.md"
---

An OCI-compliant content delivery and enrichment service for [Gemara](https://github.com/ossf/gemara) compliance artifacts. Clients can discover and download Gemara content (L1 guidance, L2 catalogs, L3 policies) as OCI artifacts using standard tooling.

### Features

- **OCI Distribution API** -- Serves Gemara compliance YAML as OCI artifacts via the standard `/v2/` registry endpoints
- **Enrichment API** -- Transforms compliance assessment results using configurable plugin mappers (`POST /v1/enrich`)
- **Content-addressable storage** -- Blobs stored on filesystem by SHA-256 digest, metadata indexed in embedded BBolt

⚠️ **NOTE:**
To disable JWT when you build the tool for local running, ensure `jwtAuth` is set to `false` in [config.yaml](https://github.com/complytime/gemara-content-service/blob/main/hack/demo/config.yaml).

### Quick Start

#### Build

```bash
make build
```

#### Run Locally (no Tls)

```bash
./bin/compass --skip-tls --port 9090
```

#### Build Container Image Locally

```shell
podman build -f images/Containerfile.compass -t gemara-content-service:local .
```


#### Run Tests

```bash
make test
```

#### Generate Self-signed Certificates For Testing

Refer to [this](https://github.com/complytime/complytime-collector-components/blob/main/Makefile#L124) 

### Project Structure

```
cmd/compass/          Main entry point and server wiring
api/                  OpenAPI-generated types and server interface
internal/             Internal packages (logging, middleware)
mapper/               Enrichment plugin framework
service/              Core enrichment service logic
images/               Container build files
hack/                 Development utilities and sample data
docs/                 Configuration files
```

### Development

#### Prerequisites

- Go 1.24+
- [golangci-lint](https://golangci-lint.run/) (optional, for `make golangci-lint`)
- [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) (for `make api-codegen`)

#### Useful Make Targets

| Target               | Description                         |
|----------------------|-------------------------------------|
| `make build`         | Build the binary                    |
| `make test`          | Run tests with coverage             |
| `make test-race`     | Run tests with race detection       |
| `make golangci-lint` | Run linter                          |
| `make api-codegen`   | Regenerate OpenAPI types and server |
| `make help`          | Show all targets                    |

### License

[Apache 2.0](https://github.com/complytime/gemara-content-service/blob/main/LICENSE)
