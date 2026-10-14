# Dev Config Specification

## Purpose

Configure a local service for development. The `just config` recipe drives the `abnchctl` developer CLI, which writes the local service URL and the local auth outside version control. A human or an agent writes the Postgres connection details. The local service config and its credentials are defined in the [Dev Config Design](../../docs/design/dev-config.md), and the `abnchctl` commands in the [Dev Tooling Design](../../docs/design/app/abnchctl.md).

## Requirements

### Requirement: Config Location

`abnchctl` SHALL read and write the local service config at `.abnchd/config.yaml` under the repo root, and SHALL keep the file out of version control.

The `--config` option SHALL override the path for both `set local-auth` and `check config`.

#### Scenario: Default path

- GIVEN a repo and no `--config` override
- WHEN `abnchctl set local-auth` runs
- THEN it writes the config to `.abnchd/config.yaml` under the repo root
- AND git ignores the file

#### Scenario: Path override

- GIVEN the user passes `--config` a path
- WHEN `abnchctl set local-auth` or `abnchctl check config` runs
- THEN it uses that path instead of `.abnchd/config.yaml`

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

### Requirement: Auth Only

`abnchctl set local-auth` SHALL write only local auth config. It SHALL NOT prompt for or change the Postgres config.

#### Scenario: Postgres config is preserved

- GIVEN a local service config holds a Postgres block
- WHEN `abnchctl set local-auth` finishes
- THEN the Postgres block is unchanged

### Requirement: Local Auth

`abnchctl set local-auth` SHALL generate a dev key, a dev issuer, and a local CLI credential for the local service URL. The dev key SHALL live beside the local service config, so the config and the key that signs its issuer move together.

#### Scenario: Generate local auth

- GIVEN the user runs `just config`
- WHEN `abnchctl set local-auth` finishes
- THEN it generates a dev key
- AND it adds the dev issuer to the local service config
- AND it writes a local CLI credential keyed by the local service URL

#### Scenario: Dev key beside the config

- GIVEN the user runs `just config`
- WHEN `abnchctl set local-auth` writes the dev key
- THEN it writes the key to `.abnchd/dev-issuer.key` under the repo root
- AND git ignores the key through the `.abnchd` directory

#### Scenario: Dev key follows a config override

- GIVEN the user passes `--config` a path
- WHEN `abnchctl set local-auth` writes the dev key
- THEN it writes the key beside that config file

### Requirement: Check Config

After `abnchctl set local-auth` succeeds, `just config` SHALL run `abnchctl check config` to confirm the Postgres config is present. The check SHALL expect the config to already exist and SHALL NOT create or change it.

#### Scenario: Confirm the config

- GIVEN `abnchctl set local-auth` wrote the local auth config
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
