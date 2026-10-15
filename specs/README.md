# Specs

Behavior contracts for the Agent Benchmark system.

## Service

- [Auth](service/auth.md) — authenticating callers to the service API and granting them roles
- [Repo](service/repo.md) — managing repos over the service API, and the conventions other resources share
- [Task](service/task.md) — managing tasks within a repo over the service API
- [Test](service/test.md) — managing tests within a task over the service API
- [Data](service/data.md) — managing a test's data over the service API
- [Result](service/result.md) — storing and reading the results agents produce

## CLI

- [Auth](cli/auth.md) — signing a human user in and out, storing a session, and presenting credentials
- [Repo](cli/repo.md) — deriving the repo a command acts on from the working directory
- [Task](cli/task.md) — creating, updating, and reading tasks
- [Test](cli/test.md) — creating, updating, and reading tests
- [Data](cli/data.md) — adding and reading a test's data
- [Model](cli/model.md) — recording the models an agent may call
- [Agent](cli/agent.md) — recording a task's agent
- [Run](cli/run.md) — running an agent against a test's data and reading results

## Dev

- [Config](dev/config.md) — configuring a local service for development
