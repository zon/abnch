# CLI Design

## `set` Commands

`set` commands update if a matching resource already exists or create if the resource is missing. Updates are peformed by merging values passed to the command with the resource.

`set` commands raise an error if a required field is missing on both the resource and input.

## `abnch set config`

Create or updates an `.abnch/config.yaml` file in the repo under the working dir.

### Config Schema

```yaml
url: string - abnch service url
```

## `abnch set model <provider>/<name>`

Create or updates model config. Adds a model option to the config file.

* --stub: optional string - override the auto generated stub
* --variant: optional string

### Config Schema

```yaml
models:
  <stub>:
    name: string
    provider: string
    variant: string
```

## `abnch set agent <task stub>`

Creates or updates agent config for a task.

* --version-cmd: string
* --build-cmd: string
* --run-cmd: string

### Config Schema

```yaml
tasks:
  <task stub>:
    agent:
      versionCmd: string
      buildCmd: string
      runCmd: string
```

## `abnch set grader <task stub>`

Creates or updates grader config for a task.

* --version-cmd: string
* --build-cmd: string
* --run-cmd: string

### Config Schema

```yaml
tasks:
  <task stub>:
    grader:
      versionCmd: string
      buildCmd: string
      runCmd: string
```

## `abnch set task <stub>`

Create or updates a task. Can recieve options or a YAML file with matching properties as stdin. The repo property is inferred from the working dir.

* --name: string - task name
* --description: string - task description
* --move: optional string - renames task from <stub> to --move value
* --model: optional string - sets the task's default model in config

### Config Schema

```yaml
tasks:
  <task stub>:
    model: <model stub>
```

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

## `abnch run <task stub>/<test stub>` | `abnch run <task stub>/<test stub>/<data stub>`

Run a test.

* --model: string - override default model
* --local: bool - run locally instead of using a workflow
* --limit: int - cap data iterations
* --shuffle: bool - randomly sort iterations

## `abnch score <result ref>`

Score a result.

* --model: string - override default model
* --local: bool - run locally instead of using a workflow
