# Deployment Design

Specific deployments are expected to provide access to any model providers or file APIs needed.

## Secrets

Agent and grader workflows can load files and environment variables from Kubernetes secrets. Specific deployments are responsible for configuring what their agents and graders need beyond the service account token.