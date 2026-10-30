# CLI Config Design

The CLI creates and updates an `.abnch/config.yaml` file in the repo under the working dir.

## Schema

```yaml
url: string - abnchd service url
```

## Commands

`abnch set config` creates or updates the service URL, and `abnch get config` reads the config. Result and score submission carry their own model, agent, and grader records, so the config holds only the service URL.
