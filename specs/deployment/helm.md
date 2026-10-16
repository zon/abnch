# Helm Chart Specification

## Purpose

Install the service into a Kubernetes cluster. The chart supplies the [Service Config](../../docs/design/app/abnchd-config.md) through its values and runs the service image the [Container spec](container.md) delivers, tagged with the [shared version](../version.md). The [Deployment Design](../../docs/design/deployment.md) describes the chart and the secrets a deployment supplies.

## Requirements

### Requirement: Chart Installation

The repo SHALL provide a Helm chart that installs `abnchd` into a Kubernetes cluster.

#### Scenario: Install the service

- GIVEN a cluster and the repo's chart
- WHEN a caller installs the chart
- THEN the cluster runs the service image
- AND the service is reachable through a Kubernetes Service
- AND no installation input beyond the chart and its values is required

#### Scenario: Uninstall the service

- GIVEN an installed chart
- WHEN the caller uninstalls it
- THEN its workload and Service are removed

### Requirement: Supplied Service Config

The chart SHALL render a service config from its values and run `abnchd` from that config. The values SHALL cover the service URL, the Postgres connection, and the OIDC and workload auth settings the [Service Config Design](../../docs/design/app/abnchd-config.md) describes.

#### Scenario: Values supply the config

- GIVEN chart values for the service URL, Postgres, and auth
- WHEN the chart renders
- THEN it writes a service config holding those values
- AND it starts `abnchd` with that config's path
- AND the service uses those values

#### Scenario: Incomplete values

- GIVEN chart values missing a value the service config requires
- WHEN the chart installs
- THEN the install fails or the service reports the error
- AND the chart does not silently install an unusable service

### Requirement: Secret Values

The chart SHALL read a secret value from a Kubernetes secret rather than from chart values. The Postgres password and the OIDC client secret SHALL come from a secret.

#### Scenario: Secret-backed config

- GIVEN a Kubernetes secret holding the Postgres password and the OIDC client secret
- WHEN the chart renders
- THEN the rendered service config holds those secret values
- AND the chart values hold the secret's name and keys, not the values themselves

### Requirement: Image Version

The chart SHALL run the service image tagged with the repo version. The image repository and tag SHALL be chart values that default to the service image at the [shared version](../version.md).

#### Scenario: Version tag

- GIVEN the repo version
- WHEN the chart renders with its default values
- THEN the workload names the service image at that version

#### Scenario: Override the tag

- GIVEN a chart value naming another image tag
- WHEN the chart renders
- THEN the workload names the image at that tag

### Requirement: Chart Validity

The chart SHALL lint clean and SHALL render valid Kubernetes manifests with its default values.

#### Scenario: Lint the chart

- GIVEN the repo's chart
- WHEN `helm lint` runs
- THEN it reports no error

#### Scenario: Render the chart

- GIVEN the repo's chart
- WHEN the chart renders with its default values
- THEN it produces valid Kubernetes manifests
