# Dev Config Specification

## Purpose

Configure a local service for development. The `just config` recipe drives the `abnchctl` developer CLI, which does the work: it collects the local service values, writes them outside version control, and generates local auth. The local service config and its credentials are defined in the [Dev Config Design](../../docs/design/dev-config.md), and the `abnchctl` commands in the [Dev Tooling Design](../../docs/design/app/abnchctl.md).

## Requirements

### Requirement: Service Port

`just config` SHALL take an optional local service port and pass it to `abnchctl set local-auth`. The port SHALL set the local service URL, and `just config` SHALL use `8080` when no port is given.

#### Scenario: Default port

- GIVEN the user gives no port
- WHEN the user runs `just config`
- THEN the local service URL uses port `8080`

#### Scenario: Port given

- GIVEN the user runs `just config` with a port
- WHEN `abnchctl set local-auth` builds the local service URL
- THEN the local service URL uses that port

### Requirement: Postgres Config

`abnchctl set local-auth` SHALL prompt for each Postgres field (host, port, dbname, user, password, sslmode, and timeZone) and write the local service config. The password prompt SHALL hide typing, sslmode SHALL default to `disable`, and timeZone SHALL default to `UTC`. A value already in the config SHALL become the prompt default.

#### Scenario: Write the config

- GIVEN the user answers each Postgres prompt
- WHEN `abnchctl set local-auth` finishes
- THEN it writes the local service config
- AND it reports the file path

#### Scenario: Re-run

- GIVEN a local service config exists
- WHEN the user runs `just config` again
- THEN each existing value is the prompt default

### Requirement: Local Auth

`abnchctl set local-auth` SHALL generate a dev key, a dev issuer, and a local CLI credential for the local service URL.

#### Scenario: Generate local auth

- GIVEN the user runs `just config`
- WHEN `abnchctl set local-auth` finishes
- THEN it generates a dev key
- AND it adds the dev issuer to the local service config
- AND it writes a local CLI credential keyed by the local service URL

### Requirement: Check Config

After `abnchctl set local-auth` succeeds, `just config` SHALL run `abnchctl check config` to confirm the Postgres config is present. The check SHALL expect the config to already exist and SHALL NOT create or change it.

#### Scenario: Confirm the config

- GIVEN `abnchctl set local-auth` wrote the Postgres config
- WHEN the user runs `just config`
- THEN the recipe runs `abnchctl check config`
- AND it confirms the Postgres config is present

#### Scenario: Missing config

- GIVEN the Postgres config is not present
- WHEN `abnchctl check config` runs
- THEN it fails
- AND it reports that the Postgres config is missing

### Requirement: Command Failure

`just config` SHALL exit non-zero and report the failure when `abnchctl set local-auth` or `abnchctl check config` fails.

#### Scenario: abnchctl fails

- GIVEN a required `abnchctl` command fails
- WHEN the recipe runs it
- THEN the recipe exits non-zero
- AND it reports the failure
