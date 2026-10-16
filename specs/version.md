# Version Specification

## Purpose

Report the semantic version the repo's apps share. One version covers `abnch`, `abnchd`, and `abnchctl`, and tags the service image. The [Container spec](deployment/container.md) delivers that image.

## Requirements

### Requirement: Shared Version

The repo SHALL define one semantic version for `abnch`, `abnchd`, and `abnchctl`. The version SHALL name a semantic version of the form `<major>.<minor>.<patch>`.

#### Scenario: Every app reports one version

- GIVEN the repo
- WHEN an app reports its version
- THEN it prints the repo's version
- AND the other apps print the same version

### Requirement: Version Flag

Each app SHALL print the repo version to stdout and exit zero when given `--version`. A version request SHALL NOT need a config, a repo, or a service.

#### Scenario: Report the version from the CLI

- GIVEN the `abnch` CLI
- WHEN the user runs `abnch --version`
- THEN it prints the repo version to stdout
- AND it exits zero

#### Scenario: Report the version from the service

- GIVEN the `abnchd` service
- WHEN the caller runs `abnchd --version`
- THEN it prints the repo version to stdout
- AND it exits zero without reading a config

#### Scenario: Report the version from the dev tool

- GIVEN the `abnchctl` dev tool
- WHEN the user runs `abnchctl --version`
- THEN it prints the repo version to stdout
- AND it exits zero

### Requirement: Image Tag

The delivery pipeline SHALL tag the service image with the repo version.

#### Scenario: Tag the image with the version

- GIVEN the delivery pipeline
- WHEN it builds the service image
- THEN it tags the image with the repo version
- AND it pushes that tag after a clean scan
