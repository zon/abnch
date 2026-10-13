# Specs

Behavior contracts for the Agent Benchmark system.

## Service

- [Auth](service/auth.md) — authenticating callers to the service API and granting them roles
- [Repo](service/repo.md) — managing repos over the service API, and the conventions other resources share
- [Task](service/task.md) — managing tasks within a repo over the service API

## CLI

- [Auth](cli/auth.md) — signing a human user in and out, storing a session, and presenting credentials
- [Repo](cli/repo.md) — deriving the repo a command acts on from the working directory
- [Task](cli/task.md) — creating, updating, and reading tasks

## Dev

- [Config](dev/config.md) — configuring a local service for development
