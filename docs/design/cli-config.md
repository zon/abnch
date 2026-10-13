# CLI Config Design

The CLI creates and updates an `.abnch/config.yaml` file in the repo under the working dir.

## Schema

```yaml
url: string - abnch service url
models:
  <stub>:
    name: string
    provider: string
    variant: string
tasks:
  <task stub>:
    model: <model stub>
    agent:
      versionCmd: string
      buildCmd: string
      runCmd: string
    grader:
      versionCmd: string
      buildCmd: string
      runCmd: string
```

## Commands

The `set` commands in the [CLI Design](cli.md) write these values. The `get` commands read them.
