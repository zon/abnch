# Dev Config Specification

## Purpose

Configure a local service for development. The `just config` recipe collects the values a local service needs and writes them outside version control. The local service config and its credentials are defined in the [Dev Config Design](../../docs/design/dev-config.md).

## Requirements

### Requirement: Service Port

`just config` SHALL take an optional local service port and SHALL use `8080` when no port is given. The port SHALL set the local service URL.

#### Scenario: Default port

- GIVEN the user gives no port
- WHEN the user runs `just config`
- THEN the local service URL uses port `8080`

#### Scenario: Port given

- GIVEN the user runs `just config` with a port
- WHEN the recipe builds the local service URL
- THEN the local service URL uses that port

### Requirement: Postgres Config

`just config` SHALL prompt for each Postgres field and write a local service config. A value already in the config SHALL become the prompt default.

#### Scenario: Write the config

- GIVEN the user answers each Postgres prompt
- WHEN the recipe finishes
- THEN it writes the local service config
- AND it reports the file path

#### Scenario: Re-run

- GIVEN a local service config exists
- WHEN the user runs `just config` again
- THEN each existing value is the prompt default

### Requirement: Local Credentials

`just config` SHALL generate a dev key, a dev issuer, and a local CLI credential for the local service URL.

#### Scenario: Generate credentials

- GIVEN the user runs `just config`
- WHEN the recipe finishes
- THEN it generates a dev key
- AND it adds the dev issuer to the local service config
- AND it writes a local CLI credential keyed by the local service URL
