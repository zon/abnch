# Deployment Design

Specific deployments are expected to provide access to any model providers or file APIs needed.

## Container Images

The repo ships its apps as container images that follow the [Container Standard](../zpecs/containers.md). The service image runs `abnchd` from a [service config](app/abnchd-config.md), which a deployment supplies as a file. A GitHub Actions workflow builds, scans, and pushes the images to the GitHub Container Registry under `ghcr.io/zon/abnch`, tagged with the repo's semantic version.

## Helm Chart

The service provides a Helm chart at `deploy/helm/abnchd` that installs it into a Kubernetes cluster. The chart values supply the [Service Config](app/abnchd-config.md). The chart version and app version are the repo's semantic version, so a chart runs the image of the same version by default.

The same workflow that pushes the image publishes the chart as an OCI artifact at `ghcr.io/zon/abnch/charts/abnchd`, tagged with the repo version, once the image at that version is pushed. The chart package is public, like the image, so a deployment installs it with `oci://ghcr.io/zon/abnch/charts` as the repository, `abnchd` as the chart, and the repo version. A version is published once. A push to `main` that does not bump the version publishes nothing.

## Secrets

Agent and grader workflows can load files and environment variables from Kubernetes secrets. Specific deployments are responsible for configuring what their agents and graders need beyond the service account token.