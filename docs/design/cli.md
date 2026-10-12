# CLI Design

## `abnch set config`

Create or updates an `.abnch/config.yaml` file in the repo under the working dir. Interactively collects any required missing values.

### Config Schema

```yaml
url: string - abnch service url
```

## `abnch set task`

Create or updates a task. Can recieve options or a YAML file with matching properties as stdin. The repo property is inferred from the working dir.

* --stub: optional string - task stub
* --name: string - task name
* --description: string - task description
* --move: optional string - renames task from --stub to --move value

## `abnch get tasks` | `abnch get task`

List tasks in the repo.

## `abnch get task <stub>`

Outputs a task as YAML.