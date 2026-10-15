# Dev Config Specification

## Purpose

Configure a local service for development. The `just config` recipe drives the `abnchctl` developer CLI, which writes the local service URL and the local auth outside version control. A human or an agent writes the Postgres connection details. The local service config and its credentials are defined in the [Dev Config Design](../../docs/design/dev-config.md), and the `abnchctl` commands in the [Dev Tooling Design](../../docs/design/app/abnchctl.md).

## Requirements

### Requirement: Config Location

`abnchctl` SHALL read and write the local service config at `.abnchd/config.yaml` under the repo root, and SHALL keep the file out of version control.

The `--config` option SHALL override the path for `set local-auth`, `set ralph`, and `check config`.

#### Scenario: Default path

- GIVEN a repo and no `--config` override
- WHEN `abnchctl set local-auth` runs
- THEN it writes the config to `.abnchd/config.yaml` under the repo root
- AND git ignores the file

#### Scenario: Path override

- GIVEN the user passes `--config` a path
- WHEN `abnchctl set local-auth`, `abnchctl set ralph`, or `abnchctl check config` runs
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

### Requirement: CLI Config

`abnchctl set local-auth` SHALL write the local service URL to the [CLI config](../../docs/design/app/abnch-config.md) at `.abnch/config.yaml` under the repo root, so the `abnch` CLI reaches the local service without `--url`. The write SHALL preserve the file's other keys.

#### Scenario: Write the CLI config

- GIVEN the user runs `just config`
- WHEN `abnchctl set local-auth` finishes
- THEN `.abnch/config.yaml` holds the local service URL

#### Scenario: Keep other CLI config keys

- GIVEN `.abnch/config.yaml` holds models and tasks
- WHEN `abnchctl set local-auth` finishes
- THEN the local service URL is updated
- AND the models and tasks are unchanged

### Requirement: Ralph Config Publication

`abnchctl set ralph` SHALL publish the local dev config to the Ralph namespace for remote runs. It SHALL write the local service config and the dev key beside it to the `abnchd-config` Secret, and the CLI config to the `abnch-config` ConfigMap.

It SHALL set the published service config's Postgres host and port to the address a run inside the cluster uses, so a remote run reaches the database the local config reaches by a dev address. The host SHALL default to `postgres-rw.postgres` and the port to `5432`. The `--postgres-host` and `--postgres-port` options SHALL override them. It SHALL leave the local service config unchanged.

It SHALL resolve the namespace from `workflow.namespace` in `.ralph/config.yaml` and the context from `workflow.context`. The `--namespace` and `--context` options SHALL override them. It SHALL fail when no namespace is set.

It SHALL NOT change `.ralph/config.yaml`. The repo's config declares the mounts.

#### Scenario: Publish

- GIVEN a repo with a local service config, a dev key, a CLI config, and `.ralph/config.yaml` setting `workflow.namespace: ralph-abnch`
- WHEN `abnchctl set ralph` runs
- THEN it writes the `abnchd-config` Secret with the published service config and the dev key
- AND it writes the `abnch-config` ConfigMap with the CLI config
- AND `.ralph/config.yaml` is unchanged

#### Scenario: Reach Postgres from the cluster

- GIVEN a local service config whose Postgres host and port are a dev address
- WHEN `abnchctl set ralph` runs
- THEN the published config's Postgres host is `postgres-rw.postgres`
- AND its Postgres port is `5432`
- AND its other Postgres values are unchanged
- AND the local service config is unchanged

#### Scenario: Postgres flags

- GIVEN a local service config
- WHEN `abnchctl set ralph --postgres-host db.internal --postgres-port 5433` runs
- THEN the published config's Postgres host is `db.internal`
- AND its Postgres port is `5433`

#### Scenario: Flags target another namespace

- GIVEN `.ralph/config.yaml` sets `workflow.namespace: ralph-abnch`
- WHEN `abnchctl set ralph --namespace staging --context canary` runs
- THEN it publishes to the `staging` namespace in the `canary` context

#### Scenario: Namespace missing

- GIVEN `.ralph/config.yaml` sets no `workflow.namespace`
- WHEN `abnchctl set ralph` runs without `--namespace`
- THEN it fails
- AND it reports that no namespace is set

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

### Requirement: Service Recipe

`just service` SHALL start `abnchd` against the local service config at `.abnchd/config.yaml`.

#### Scenario: Start the service

- GIVEN `just config` wrote the local service config
- WHEN the user runs `just service`
- THEN it starts `abnchd` with `.abnchd/config.yaml`
- AND the service connects to Postgres with the details in the file

### Requirement: Command Failure

`just config` SHALL exit non-zero and report the failure when `abnchctl set local-auth` or `abnchctl check config` fails.

#### Scenario: abnchctl fails

- GIVEN a required `abnchctl` command fails
- WHEN the recipe runs it
- THEN the recipe exits non-zero
- AND it reports the failure
