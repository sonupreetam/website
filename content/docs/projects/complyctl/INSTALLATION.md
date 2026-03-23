---
title: "Installation"
description: "A command-line tool for streamlining end-to-end compliance workflows on local systems. — Installation"
date: 2026-03-23T09:20:36Z
lastmod: 2026-03-23T09:20:36Z
draft: false
weight: 10
params:
  editURL: "https://github.com/complytime/complyctl/edit/main/docs/INSTALLATION.md"
---
<!-- synced from complytime/complyctl/docs/INSTALLATION.md@main (3bd616905897) -->

### Binary

The latest binary release can be downloaded from <https://github.com/complytime/complyctl/releases/latest>.

Verify the release signature:

```bash
cosign verify-blob \
  --certificate complyctl_*_checksums.txt.pem \
  --signature complyctl_*_checksums.txt.sig \
  complyctl_*_checksums.txt \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
  --certificate-identity=https://github.com/complytime/complyctl/.github/workflows/release.yml@refs/heads/main
```

### From Source

#### Prerequisites

- **Go** 1.24+
- **Make**
- **buf** CLI (optional, for protobuf regeneration)

#### Clone And Build

```bash
git clone https://github.com/complytime/complyctl.git
cd complyctl
make build
```

Binaries are placed in `bin/`. Add it to your `PATH`:

```bash
export PATH="$PWD/bin:$PATH"
```

#### Build The Test Plugin (optional)

```bash
make build-test-plugin
```

Produces `bin/complyctl-provider-test` for use in E2E testing. See [E2E_INTEGRATION.md](https://github.com/complytime/complyctl/blob/main/docs/E2E_INTEGRATION.md).
