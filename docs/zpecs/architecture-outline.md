# Architecture Format

The architecture format lists, in YAML, the components a project is built from.

## Location

The architecture document lives at **`/specs/architecture.yaml`** and describes the **current** components of the application.

## Format

The architecture document is a YAML list. A component is a collection of resources that serve one concern: source code, scripts, runtimes, assets, or config. Each entry has the following fields:

- **path** (required, string): The file path or directory path where the component lives. Relative to the repo root.
- **description** (required, string): A high-level overview of the component in one short sentence. State what it is for, not what it contains. Do not list behaviors, method names, routes, interfaces, or error types. Details like these churn every time the component grows.

## Example

```yaml
- path: cmd/myapp
  description: Wires dependencies and starts the server.

- path: internal/orders
  description: Coordinates the order lifecycle.

- path: internal/postgres
  description: Stores domain data in PostgreSQL.

- path: internal/httpapi
  description: Exposes the domain over HTTP.

- path: deploy/containers
  description: Builds and configures the deployed containers.
```

## How the Architecture Is Used

The architecture document is the component map. It records what exists, so write it after the code and update it as the code changes. Before writing code, read it to find where the code belongs ([Architecture Guidelines](architecture.md)).

Keeping it accurate matters more than keeping it complete. A component whose description no longer matches what it does is worse than a component that is missing.
