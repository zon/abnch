# Deployment Design

Specific deployments are expected to provide access to any model providers or file APIs needed.

## Container Images

The repo ships its apps as container images that follow the [Container Standard](../zpecs/containers.md). The service image runs `abnchd` from a [service config](app/abnchd-config.md), which a deployment supplies as a file. A GitHub Actions workflow builds, scans, and pushes the images to the GitHub Container Registry under `ghcr.io/zon/abnch`, tagged with the repo's semantic version.

## Transport Security

The service serves HTTPS and terminates TLS itself. A deployment issues the certificate with cert-manager. The Helm chart renders the service Certificate, signs it with an issuer the deployment provides, mounts the certificate Secret, and configures `abnchd` to serve it. See [HTTPS Design](https.md).

A deployment that publishes the service outside the cluster enables the chart's ingress. It renders a Gateway API TLSRoute attached to an existing Gateway, which forwards TLS by SNI without terminating it.

The cluster must have cert-manager installed before the chart. The chart does not install it.

The certificate Secret belongs to cert-manager, not to the chart values. The chart names it and mounts it.

## Helm Chart

The service provides a Helm chart at `deploy/helm/abnchd` that installs it into a Kubernetes cluster. The chart values supply the [Service Config](app/abnchd-config.md), including the TLS certificate paths. The chart version and app version are the repo's semantic version, so a chart runs the image of the same version by default.

The same workflow that pushes the image publishes the chart as an OCI artifact at `ghcr.io/zon/abnch/charts/abnchd`, tagged with the repo version, once the image at that version is pushed. The chart package is public, like the image, so a deployment installs it with `oci://ghcr.io/zon/abnch/charts` as the repository, `abnchd` as the chart, and the repo version. A version is published once. A push to `main` that does not bump the version publishes nothing.