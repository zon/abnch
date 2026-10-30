# CLI Config Design

The CLI creates and updates an `.abnch/config.yaml` file in the repo under the working dir.

## Schema

```yaml
url: string - abnchd service url
models:
  <stub>:
    name: string
    provider: string
    variant: string
tasks:
  <task stub>:
    agent:
      name: string
      stub: string
      commit: string
      branch: string
      version: string
      uncommitted: bool
    grader:
      name: string
      stub: string
      commit: string
      branch: string
      version: string
      uncommitted: bool
```

## Commands

The `add result` and `add score` commands record the model, agent, and grader they use in the config. The model stub is derived from the model's name. The `get` commands read them. The config keeps the last record seen for a model stub, a task's agent, and a task's grader.
