# CLI Design

## `set` Commands

`set` commands update if a matching resource already exists or create if the resource is missing. Updates are peformed by merging values passed to the command with the resource.

## `abnch set config`

Create or updates an `.abnch/config.yaml` file in the repo under the working dir. Interactively collects any required missing values.

### Config Schema

```yaml
url: string - abnch service url
```

## `abnch set task <stub>`

Create or updates a task. Can recieve options or a YAML file with matching properties as stdin. The repo property is inferred from the working dir.

* --name: string - task name
* --description: string - task description
* --move: optional string - renames task from <stub> to --move value

## `abnch get tasks` | `abnch get task`

List tasks in the repo.

## `abnch get task <stub>`

Outputs a task as YAML.

## `abnch set test <task stub>/<stub>`

Create or updates a test. Can receive options or a YAML file with matching properties as stdin.

* --name: string - test name
* --description: string - test description
* --version: string - test version
* --move: optional string - renames test from <stub> to --move value

The schema keys are set through the YAML input.

## `abnch get tests` | `abnch get test`

List tests.

## `abnch get test <stub>`

Outputs a test as YAML.

## `abnch get data <task stub>/<test stub>`

Lists a test's data.

## `abnch get data <task stub>/<test stub>/<stub>`

Outputs a datum as JSON.

## `abnch add data <task stub>/<test stub>`

Adds test data. Expects stdin or `--file` containing valid JSON.

* --file: relative path - a valid JSON file