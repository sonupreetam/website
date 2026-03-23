---
title: "Development"
description: "A policy-driven observability toolkit for compliance evidence collection — Development"
date: 2026-03-23T08:53:23Z
lastmod: 2026-03-23T08:53:23Z
draft: false
weight: 10
params:
  editURL: "https://github.com/complytime/complytime-collector-components/edit/main/docs/DEVELOPMENT.md"
---
<!-- synced from complytime/complytime-collector-components/docs/DEVELOPMENT.md@main (33bc511c1043) -->

This guide provides comprehensive instructions for setting up, building, and testing the ComplyBeacon project.
It complements the [DESIGN.md](https://github.com/complytime/complytime-collector-components/blob/main/docs/DESIGN.md) document by focusing on the practical aspects of development.

<!-- TOC -->
* [ComplyBeacon Development Guide](#complybeacon-development-guide)
  * [Prerequisites](#prerequisites)
    * [Required Software](#required-software)
  * [Development Environment Setup](#development-environment-setup)
    * [1. Clone the Repository](#1-clone-the-repository)
    * [2. Install podman-compose (if needed)](#2-install-podman-compose-if-needed)
    * [3. Initialize Go Workspace](#3-initialize-go-workspace)
    * [4. Install Dependencies](#4-install-dependencies)
    * [5. Verify Installation](#5-verify-installation)
  * [Project Structure](#project-structure)
  * [Testing](#testing)
    * [Running Tests](#running-tests)
    * [Integration Testing](#integration-testing)
  * [Component Development](#component-development)
    * [1. ProofWatch Development](#1-proofwatch-development)
    * [2. Compass Development](#2-compass-development)
    * [3. TruthBeam Development](#3-truthbeam-development)
    * [4. Beacon Distro Development](#4-beacon-distro-development)
  * [Debugging and Troubleshooting](#debugging-and-troubleshooting)
    * [Debugging Tools](#debugging-tools)
  * [Code Generation](#code-generation)
    * [1. OpenTelemetry Semantic Conventions](#1-opentelemetry-semantic-conventions)
    * [2. Manual Code Generation](#2-manual-code-generation)
  * [Deployment and Demo](#deployment-and-demo)
    * [Local Development Demo](#local-development-demo)
  * [Additional Resources](#additional-resources)
<!-- TOC -->

### Prerequisites

#### Required Software

- **Go 1.24+**: The project uses Go 1.24.0 with toolchain 1.24.5
- **Podman**: For containerized development and deployment
- **podman-compose**: For orchestrating multi-container development environments
- **Make**: For build automation
- **Git**: For version control
- **openssl** Cryptography toolkit 

### Development Environment Setup

#### 1. Clone The Repository

```bash
git clone https://github.com/complytime/complybeacon.git
cd complybeacon
```

#### 2. Install Podman-compose (if Needed)

The project uses `podman-compose` for container orchestration. Install it if you don't have it:

```bash
## Install Podman-compose
pip install podman-compose

## Alternatively For Fedora:
dnf install podman-compose

## Verify Installation
podman-compose --version
```

#### 3. Initialize Go Workspace

The project uses Go workspaces to manage multiple modules:

```bash
make workspace
```

This creates a `go.work` file that includes all project modules:
- `./proofwatch`
- `./truthbeam`

#### 4. Install Dependencies

Dependencies are managed per module. Install them for all modules:

```bash
## Install Dependencies For All Modules
for module in proofwatch truthbeam; do
    cd $module && go mod download && cd ..
done
```

#### 5. Verify Installation

```bash
## Run Tests To Verify Everything Works
make test

## Build All Binaries
make build
```

### Project Structure

```
complybeacon/
├── compose.yaml                # podman-compose configuration for demo environment
├── Makefile                    # Build automation
├── docs/                       # Documentation
│   ├── DESIGN.md              # Architecture and design documentation
│   ├── DEVELOPMENT.md         # This file
│   └── attributes/            # Attribute documentation
├── model/                      # OpenTelemetry semantic conventions
│   ├── attributes.yaml        # Attribute definitions
│   └── entities.yaml          # Entity definitions
├── proofwatch/                 # ProofWatch instrumentation library
│   ├── attributes.go          # Attribute definitions
│   ├── evidence.go            # Evidence types
│   └── proofwatch.go          # Main library
├── truthbeam/                  # TruthBeam processor module
│   ├── internal/              # Internal packages
│   ├── config.go              # Configuration
│   └── processor.go           # Main processor logic
├── beacon-distro/              # OpenTelemetry Collector distribution
│   ├── config.yaml            # Collector configuration
│   └── Containerfile.collector # Container definition
├── hack/                       # Development utilities
│   ├── demo/                  # Demo configurations
│   ├── sampledata/            # Sample data for testing
│   └── self-signed-cert/      # self signed cert, testing/development purpose
└── bin/                        # Built binaries (created by make build)
```

### Testing

#### Running Tests

```bash
## Run All Tests
make test

## Run Tests For Specific Module
cd proofwatch && go test -v ./...
cd truthbeam && go test -v ./...
```

#### Integration Testing

The project includes integration tests using the demo environment:

```bash
## Start The Demo Environment
make deploy

## Test The Pipeline
curl -X POST http://localhost:8088/eventsource/receiver \
  -H "Content-Type: application/json" \
  -d @hack/sampledata/evidence.json

## Check Logs In Grafana At Http://localhost:3000
## Check Compass API At Http://localhost:8081/v1/enrich
```

### Component Development

#### 1. Proofwatch Development

ProofWatch is an instrumentation library for emitting compliance evidence.

**Key Files:**
- `proofwatch/proofwatch.go` - Main library interface
- `proofwatch/evidence.go` - Evidence type definition
- `proofwatch/attributes.go` - OpenTelemetry attributes

**Development Workflow:**
```bash
cd proofwatch

## Run Tests
go test -v ./...

## Check For Linting Issues
go vet ./...

## Format Code
go fmt ./...
```

#### 2. Compass Development

Compass is maintained as a separate project. See [gemara-content-service](https://github.com/complytime/gemara-content-service) for its source code, API specification, and contribution guidelines.

In the ComplyBeacon demo stack, Compass runs as a pre-built container image (`ghcr.io/complytime/gemara-content-service:latest`) defined in `compose.yaml`.

#### 3. Truthbeam Development

TruthBeam is an OpenTelemetry Collector processor for enriching logs.

**Key Files:**
- `truthbeam/processor.go` - Main processor logic
- `truthbeam/config.go` - Configuration structures
- `truthbeam/factory.go` - Processor factory

**Development Workflow:**
```bash
cd truthbeam

## Run Tests
go test -v ./...

## Test With Collector (requires Beacon-distro)
cd ../beacon-distro
## Modify Config To Use Local Truthbeam
## Run Collector With Local Processor
```

**Local development config**

If you want locally test the TruthBeam, remember to change the [manifest.yaml](https://github.com/complytime/complytime-collector-components/blob/main/docs/../beacon-distro/manifest.yaml)

Add replace directive at the end of [manifest.yaml](https://github.com/complytime/complytime-collector-components/blob/main/docs/../beacon-distro/manifest.yaml), to make sure collector use your `truthbeam` code. Default collector will use `- gomod: github.com/complytime/complybeacon/truthbeam main`

For example:
```yaml
replaces:
  - github.com/complytime/complybeacon/truthbeam => github.com/AlexXuan233/complybeacon/truthbeam 52e4a76ea0f72a7049e73e7a5d67d988116a3892
```
or
```yaml
replaces:
  - github.com/complytime/complybeacon/truthbeam => github.com/AlexXuan233/complybeacon/truthbeam main
```

#### 4. Beacon Distro Development

The Beacon distribution is a custom OpenTelemetry Collector.

**Key Files:**
- `beacon-distro/config.yaml` - Collector configuration
- `beacon-distro/Containerfile.collector` - Container definition

**Development Workflow:**
```bash
cd beacon-distro

## Build The Collector Image
podman build -f Containerfile.collector -t complybeacon-beacon-distro:latest .

## Test With Local Configuration
podman run --rm -p 4317:4317 -p 8088:8088 \
  -v $(pwd)/config.yaml:/etc/otel-collector.yaml:Z \
  complybeacon-beacon-distro:latest
```

### Debugging And Troubleshooting

#### Debugging Tools

```bash
## View Container Logs
podman-compose logs -f compass
podman-compose logs -f collector
```

### Code Generation

The project uses several code generation tools:

#### 1. Opentelemetry Semantic Conventions

Generate documentation and Go code from semantic convention models:

```bash
## Generate Documentation
make weaver-docsgen

## Generate Go Code
make weaver-codegen

## Validate Models
make weaver-check
```

#### 2. Manual Code Generation

If you modify the semantic conventions:

```bash
## Update Semantic Conventions
vim model/attributes.yaml
vim model/entities.yaml

## Regenerate Semantic Convention Code
make weaver-codegen
```

### Deployment And Demo

#### Local Development Demo

The demo environment uses `podman-compose` to orchestrate multiple containers. Ensure you have `podman-compose` installed before proceeding.

1. **Generate self-signed certificate**

Since compass and truthbeam enabled TLS by default, first we need to generate self-signed certificate for testing/development

```shell
make generate-self-signed-cert
```

2. **Start the full stack:**
```bash
make deploy
```

3. **Test the pipeline:**
```bash
curl -X POST http://localhost:8088/eventsource/receiver \
  -H "Content-Type: application/json" \
  -d @hack/sampledata/evidence.json
```

4. **View results:**
- Grafana: http://localhost:3000

5. **Stop the stack:**
```bash
make undeploy
```

---

### Additional Resources

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Go Documentation](https://golang.org/doc/)
- [Podman Documentation](https://docs.podman.io/)
- [Project Design Document](https://github.com/complytime/complytime-collector-components/blob/main/docs/DESIGN.md)
- [Attribute Documentation](https://github.com/complytime/complytime-collector-components/blob/main/docs/attributes/)
- [Containers Guide](https://github.com/complytime/community/blob/main/CONTAINERS_GUIDE.md)

For questions or support, please open an issue in the GitHub repository.
