# Specs

Behavior contracts for the Agent Benchmark system.

## Service

- [Auth](service/auth.md) — authenticating callers to the service API and granting them roles
- [HTTPS](service/https.md) — serving the service API over TLS from a certificate the deployment supplies
- [Repo](service/repo.md) — managing repos over the service API, and the conventions other resources share
- [Task](service/task.md) — managing tasks within a repo over the service API
- [Test](service/test.md) — managing tests within a task over the service API
- [Data](service/data.md) — managing a test's data over the service API
- [Provenance](service/provenance.md) — storing the model, agent, and grader records that results and scores reference
- [Result](service/result.md) — storing and reading the results agents produce
- [Score](service/score.md) — storing and reading the scores graders produce

## CLI

- [Auth](cli/auth.md) — signing a human user in and out, storing a session, and presenting credentials
- [Config](cli/config.md) — creating, updating, and reading the local CLI config
- [Repo](cli/repo.md) — deriving the repo a command acts on from the working directory
- [Task](cli/task.md) — creating, updating, and reading tasks
- [Test](cli/test.md) — creating, updating, and reading tests
- [Data](cli/data.md) — adding and reading a test's data
- [Result](cli/result.md) — adding a result for a test's datum and reading results
- [Score](cli/score.md) — adding a score for a result and reading scores

## Dev

- [Config](dev/config.md) — configuring a local service for development

## Version

- [Version](version.md) — reporting the semantic version the repo's apps share and tagging the service image

## Deployment

- [Container](deployment/container.md) — packaging and delivering the repo's apps as container images
- [Helm](deployment/helm.md) — installing the service into a Kubernetes cluster with a Helm chart
