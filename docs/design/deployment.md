# Deployment Design

Specific deployments are expected to provide access to any model providers or file APIs needed.

## Container Images

The repo ships its apps as container images that follow the [Container Standard](../zpecs/containers.md). The service image runs `abnchd` from a [service config](app/abnchd-config.md), which a deployment supplies as a file. A GitHub Actions workflow builds, scans, and pushes the images to the GitHub Container Registry under `ghcr.io/zon/abnch`.

## Helm Chart

The service provides a Helm chart that installs it into a Kubernetes cluster. The chart values supply the [Service Config](app/abnchd-config.md).

## Secrets

Agent and grader workflows can load files and environment variables from Kubernetes secrets. Specific deployments are responsible for configuring what their agents and graders need beyond the service account token.