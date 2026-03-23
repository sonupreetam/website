---
title: "C2p Openscap Manifest"
description: "A command-line tool for streamlining end-to-end compliance workflows on local systems. — C2p Openscap Manifest"
date: 2026-03-23T09:20:36Z
lastmod: 2026-03-23T09:20:36Z
draft: false
weight: 10
params:
  editURL: "https://github.com/complytime/complyctl/edit/main/docs/man/c2p-openscap-manifest.md"
---
<!-- synced from complytime/complyctl/docs/man/c2p-openscap-manifest.md@main (692545719f8e) -->

% C2P-OPENSCAP-MANIFEST.JSON(5) complyctl OpenSCAP Plugin Configuration
% Marcus Burghardt <maburgha@redhat.com>
% June 2025

## Name

c2p-openscap-manifest.json - Configuration file for the OpenSCAP plugin used by complyctl

## Description

This file defines the metadata and runtime configuration options for the `openscap-plugin`, a plugin to be used with `complyctl`.

It is a JSON-formatted file typically installed at:

**/usr/share/complyctl/plugins/c2p-openscap-manifest.json**

Some configuration options used by `openscap-plugin` can be overridden by using a drop-in file with the same name in "`/etc/complyctl/config.d/`":

**/etc/complyctl/config.d/c2p-openscap-manifest.json**

The easiest way to create a drop-in file is copying **/usr/share/complyctl/plugins/c2p-openscap-manifest.json** and defining the `default` values. Any other content can be removed to keep the drop-in file clean. See **CONFIGURATION OPTIONS** and **EXAMPLES** sections for more details.

For some specific cases, it is also possible to inform a custom configuration directory to override `/etc/complyctl/config.d`.
For example, the following command will try to locate and read custom settings from manifest files hosted in `/tmp/plugins-conf` instead of `/etc/complyctl/config.d`:

`complyctl generate --plugin-config /tmp/plugins-conf`

See complyctl(1) for more details about the available options.

## File Format

The configuration is a single JSON object with the following top-level keys:

- `metadata`: General plugin information
- `executablePath`: Name or path of the plugin binary
- `sha256`: The checksum of the binary (used for integrity checks)
- `configuration`: An array of runtime configuration options

## Fields

### Metadata

```json
{
  "id": "openscap",
  "description": "My openscap plugin",
  "version": "0.0.1",
  "types": [ "pvp" ]
}
```

### Executablepath

Path or name of the plugin binary to execute. Typically just:

```json
"executablePath": "openscap-plugin"
```

### Sha256
SHA256 checksum of the plugin binary, used for runtime verification.

### Configuration
A list of supported configuration parameters for the plugin.

Each entry includes:

- name: The name of the parameter
- description: Explanation of its purpose
- required: Whether this parameter must be provided
- default (optional): The default value if not specified

## Configuration Options

### Workspace (required)
Directory for writing plugin artifacts. The value is inherited from complyctl and cannot be modified.

### Profile (required)
The OpenSCAP profile to run for assessment. The value is inherited from complyctl and cannot be modified.

### Datastream (optional)
The OpenSCAP datastream to use. If not set, the plugin will try to determine it based on system information.

### Results (optional, Default: Results.xml)
The name of the generated results file.

### Arf (optional, Default: Arf.xml)
The name of the generated ARF file.

### Policy (optional, Default: Tailoring_policy.xml)
The name of the generated tailoring file.

## Examples

This is an example of a manifest including all information.

```json
{
  "metadata": {
    "id": "openscap",
    "description": "My openscap plugin",
    "version": "0.0.1",
    "types": [
      "pvp"
    ]
  },
  "executablePath": "openscap-plugin",
  "sha256": "17e8d0b82c9bfbe7c195505090954488175005898fc0e8da0812c112c582426c",
  "configuration": [
    {
      "name": "workspace",
      "description": "Directory for writing plugin artifacts",
      "required": true
    },
    {
      "name": "profile",
      "description": "The OpenSCAP profile to run for assessment",
      "required": true
    },
    {
      "name": "datastream",
      "description": "The OpenSCAP datastream to use. If not set, the plugin will try to determine it based on system information",
      "required": false
    },
    {
      "name": "policy",
      "description": "The name of the generated tailoring file",
      "default": "tailoring_policy.xml",
      "required": false
    },
    {
      "name": "arf",
      "description": "The name of the generated ARF file",
      "default": "arf.xml",
      "required": false
    },
    {
      "name": "results",
      "description": "The name of the generated results file",
      "default": "results.xml",
      "required": false
    }
  ]
}
```

This is an example of a drop-in file modifying the openscap files.
```json
{
  "configuration": [
    {
      "name": "policy",
      "default": "custom_tailoring_policy.xml",
    },
    {
      "name": "arf",
      "default": "custom_arf.xml",
    },
    {
      "name": "results",
      "default": "custom_results.xml",
    }
  ]
}
```

## See Also

complyctl(1), complyctl-openscap-plugin(7)

See the Upstream project at https://github.com/complytime/complyctl for more detailed documentation.
