# Helm Chart Specification

## Purpose

Install the service into a Kubernetes cluster. The chart supplies the [Service Config](../../docs/design/app/abnchd-config.md) through its values, runs the service image the [Container spec](container.md) delivers, configures HTTPS with cert-manager, and can publish the service through a Gateway API TLSRoute. The image is tagged with the [shared version](../version.md). The delivery pipeline publishes the chart to a registry so a cluster deployment installs it by name and version. The [Deployment Design](../../docs/design/deployment.md) describes the chart, where it is published, and the secrets a deployment supplies. The [HTTPS Design](../../docs/design/https.md) describes TLS at the service.

## Requirements

### Requirement: Chart Installation

The repo SHALL provide a Helm chart that installs `abnchd` into a Kubernetes cluster.

#### Scenario: Install the service

- GIVEN a cluster with cert-manager and the repo's chart
- WHEN a caller installs the chart
- THEN the cluster runs the service image
- AND the service is reachable through a Kubernetes Service over HTTPS
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

### Requirement: Schema Migration

The chart SHALL run `abnchd migrate` as an init container before the service container starts. The init container SHALL use the service image, the rendered service config, and the Postgres secret. The service container SHALL NOT migrate the database.

#### Scenario: Migrate before serving

- GIVEN chart values for the service config and Postgres secret
- WHEN the pod starts
- THEN the init container migrates the database with that config
- AND the service container starts only after the migration succeeds
- AND the service container does not migrate the database

#### Scenario: Migration fails

- GIVEN a database the init container cannot reach
- WHEN the pod starts
- THEN the init container fails
- AND the service container does not start

### Requirement: HTTPS Service

The chart SHALL configure `abnchd` to serve HTTPS and SHALL expose only the HTTPS port through the Service. It SHALL mount the certificate Secret and SHALL set the service config's certificate and key paths to the mounted files.

#### Scenario: HTTPS service

- GIVEN a cluster with cert-manager
- WHEN a caller installs the chart
- THEN `abnchd` serves HTTPS on its listen address
- AND the Service forwards its HTTPS port to the container
- AND the rendered config names the mounted certificate and key files

#### Scenario: No plaintext port

- GIVEN an installed chart
- WHEN a client sends a plaintext HTTP request to the Service
- THEN the service serves no plaintext API

### Requirement: Gateway Ingress

The chart SHALL render a Gateway API TLSRoute when its ingress values enable it. The route SHALL attach to the Gateway named in `ingress.gateway`, SHALL match `ingress.hostname`, and SHALL forward to the Service's HTTPS port. The gateway SHALL forward TLS without terminating it.

The chart SHALL fail to render when the ingress is enabled without a hostname.

#### Scenario: Publish through the gateway

- GIVEN values that enable the ingress and name a hostname and Gateway
- WHEN the chart renders
- THEN it renders a TLSRoute for that hostname
- AND the route attaches to that Gateway
- AND the route forwards to the Service's HTTPS port

#### Scenario: Ingress disabled

- GIVEN values that do not enable the ingress
- WHEN the chart renders
- THEN it renders no route
- AND the Service remains reachable inside the cluster

#### Scenario: Missing ingress hostname

- GIVEN values that enable the ingress without a hostname
- WHEN the chart renders
- THEN it fails
- AND it reports that a hostname is required

### Requirement: Certificate Resources

The chart SHALL require cert-manager and SHALL NOT install it. It SHALL render a cert-manager Certificate for the service signed by an existing issuer. The chart SHALL NOT render an Issuer or a CA. Values SHALL name the issuer in `tls.issuerRef`, and the chart SHALL fail to render when `tls.issuerRef.name` is empty.

The service certificate SHALL cover `tls.dnsNames`. When the value is empty, the certificate SHALL cover the Service's cluster DNS name. The certificate SHALL also cover an enabled ingress hostname.

#### Scenario: External issuer

- GIVEN values that name an existing issuer
- WHEN the chart renders
- THEN it renders a Certificate for the service
- AND the Certificate references that issuer
- AND the chart renders no Issuer or CA resources
- AND the Certificate covers the Service's cluster DNS name

#### Scenario: Missing issuer

- GIVEN values that name no issuer
- WHEN the chart renders
- THEN it fails
- AND it reports that an issuer name is required

### Requirement: Certificate Renewal

The chart SHALL mount the certificate Secret so a cert-manager renewal updates the files `abnchd` reads. The service SHALL serve a renewed certificate without a pod restart, as the [HTTPS Service spec](../service/https.md#requirement-certificate-renewal) requires.

#### Scenario: Renewal

- GIVEN cert-manager renews the service certificate
- WHEN it writes the new value to the Secret
- THEN the mounted files change
- AND `abnchd` serves the new certificate without a restart

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

The chart SHALL lint clean and SHALL render valid Kubernetes manifests with an external issuer named in its values.

#### Scenario: Lint the chart

- GIVEN the repo's chart and values that name an external issuer
- WHEN `helm lint` runs
- THEN it reports no error

#### Scenario: Render the chart

- GIVEN the repo's chart and values that name an external issuer
- WHEN the chart renders
- THEN it produces valid Kubernetes manifests

### Requirement: Chart Delivery

The delivery pipeline SHALL publish the chart to the container registry as an OCI artifact at the [shared version](../version.md), after it pushes the service image at that version. A published chart version SHALL NOT change. A consumer SHALL install the chart from the registry without credentials.

#### Scenario: Publish the chart

- GIVEN a change to the repo
- WHEN the pipeline pushes the service image at the repo version
- THEN it publishes the chart at the repo version
- AND the chart runs that image by default

#### Scenario: Version already published

- GIVEN the registry holds the chart at the repo version
- WHEN the pipeline runs again at that version
- THEN it publishes nothing
- AND the published chart is unchanged

#### Scenario: Image not pushed

- GIVEN a change whose image scan fails
- WHEN the pipeline runs
- THEN it publishes no chart

#### Scenario: Install from the registry

- GIVEN a cluster with no registry credentials
- WHEN a caller installs the chart by registry, name, and version
- THEN the cluster runs the service at that version
