# Docs

* [Read me](README.md) - A general description of this repo
* [Stack](docs/stack.md) - Dependencies this project uses. Keep up to date

## Zpecs

* [Milestones](docs/milestones.md) - High level objectives
* [Regressions](regressions.md) - How to organize and resolve bugs and problems found during other work
* [Code](docs/zpecs/code.md) - Read before writing code
  * [Architecture](docs/zpecs/architecture.md) - Read before planning code structure
  * [Dependencies](docs/zpecs/dependencies.md) - Read before adding or changing code dependencies
  * [Containers](docs/zpecs/containers.md) - Read before building or delivering a container image
* [Prose](docs/zpecs/prose.md) - Read before writing any kind of prose
* [Prompts](docs/zpecs/prompts.md) - Read before writing an agent prompt
* [Spec](docs/zpecs/specs.md) - Read before writing a spec
* [Project](docs/zpecs/project.md) - Read before writing a project
* [Glossary](docs/zpecs/glossary.md) - Read when a term is unclear
* [Process](docs/zpecs/process.md) - Read before planning milestones, projects, or regressions
* [Design](docs/zpecs/design.md) - Read before writing a high level design doc

## Design

High level design docs.

Read [Service Config](docs/design/app/abnchd-config.md) when setting up a local dev environment.

* [Auth](docs/design/auth.md) - Authentication, authorization roles, and role assignment
* [CLI](docs/design/app/abnch.md) - The `abnch` command line interface
* [CLI Config](docs/design/app/abnch-config.md) - The `.abnch/config.yaml` file
* [Deployment](docs/design/deployment.md) - Deployment access, secrets, and the Helm chart
* [HTTPS](docs/design/https.md) - TLS at abnchd and cert-manager in the Helm chart
* [Dev Config](docs/design/dev-config.md) - Local service config, collected by `just config`
* [Dev Tooling](docs/design/app/abnchctl.md) - The `abnchctl` developer CLI
* [Schema](docs/design/schema.md) - Repos, tasks, tests, data, models, agents, results, graders, and scores
* [Run](docs/design/run.md) - How a run executes an agent and records a result
* [Service App](docs/design/app/abnchd.md) - The `abnchd` service application
* [Service Config](docs/design/app/abnchd-config.md) - The service configuration file, including OIDC integration
