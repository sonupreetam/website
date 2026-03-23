---
title: "Testing Farm"
description: "A command-line tool for streamlining end-to-end compliance workflows on local systems. — Testing Farm"
date: 2026-03-23T09:20:36Z
lastmod: 2026-03-23T09:20:36Z
draft: false
weight: 10
params:
  editURL: "https://github.com/complytime/complyctl/edit/main/docs/TESTING_FARM.md"
---
<!-- synced from complytime/complyctl/docs/TESTING_FARM.md@main (af94ab2a0418) -->

[Testing Farm](https://packit.dev/docs/configuration/upstream/tests) is Packit's testing system.
Test execution is managed by tmt tool. 

The entry of the testing farm tests is located at [.packit.yaml](https://github.com/complytime/complyctl/blob/main/docs/../.packit.yaml), in the job named `tests`.

The `tests` job requires `copr_build` job to be built before running tests,
so the built packages are automatically installed in the testing environment.

The [Testing Farm documentation](https://packit.dev/docs/configuration/upstream/tests) gives information on how to include or modify tests.
