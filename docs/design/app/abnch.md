# CLI Design

## Credentials

Commands that reach the service authenticate a caller. The caller signs in with `abnch login`, which stores a session for the service URL. Later commands read the session and present it to the service. See the [Auth Design](auth.md).

Sessions live in a user-level `~/.abnch/credentials.yaml` file, keyed by service URL, outside any repo. A command that needs a session and finds none, or finds an expired one, fails and tells the user to run `abnch login`.

A service running locally trusts a dev issuer. `just config` writes a local CLI credential, which the CLI reads like a session. See the [Dev Config Design](dev-config.md).

## `abnch login`

Signs in a human user. The CLI runs the OIDC authorization code flow in a browser, receives the callback on a loopback address, and stores the resulting session for the service URL.

* --url: optional string - service url. Defaults to the [config](abnch-config.md) url

## `abnch logout`

Removes the stored session for the service URL.

* --url: optional string - service url. Defaults to the [config](abnch-config.md) url

## `abnch --version`

Prints the semantic version the repo's apps share and exits.

The `set test` version uses `--test-version`, so `--version` names the app version everywhere.

## `set` Commands

`set` commands update if a matching resource already exists or create if the resource is missing. Updates are peformed by merging values passed to the command with the resource.

`set` commands raise an error if a required field is missing on both the resource and input.

## `abnch set config`

Creates or updates the `.abnch/config.yaml` file in the repo under the working dir. The config holds only the service URL. See [CLI Config Design](abnch-config.md).

* --url: string - abnch service url

## `abnch get config`

Outputs the effective config as YAML.

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
* --test-version: string - test version
* --move: optional string - renames test from <stub> to --move value

The schema keys are set through the YAML input.

## `abnch get tests <task stub>` | `abnch get test <task stub>/<stub>`

List and read a task's tests.

## `abnch get data <task stub>/<test stub>`

Lists a test's data.

## `abnch get data <task stub>/<test stub>/<ref>`

Outputs a datum as JSON.

## `abnch add data <task stub>/<test stub>`

Adds test data. Expects stdin or `--file` containing valid JSON.

* --file: relative path - a valid JSON file

## `abnch add result <task stub>/<test stub>/<data ref>`

Adds a result for a datum. Expects stdin or `--file` containing a result input as YAML or JSON. See [Result Design](result.md).

* --file: relative path - a result input file

### File Schema

```YAML
model:
  name: string
  provider: string
  variant: string
agent:
  name: string
  stub: string
  commit: string - Git ref
  branch: string
  version: string - optional
  uncommitted: bool - optional, default false
body: any
logs: string
inputTokens: number
outputTokens: number
tokenCost: number
duration: number - seconds
```

## `abnch get results` | `abnch get result`

Lists results.

## `abnch get result <result ref>`

Outputs a result as JSON.

## `abnch add score <result ref>`

Adds a score for a result. Expects stdin or `--file` containing a score input as YAML or JSON. See [Score Design](score.md).

* --file: relative path - a score input file

### File Schema

```YAML
model:
  name: string
  provider: string
  variant: string
grader:
  name: string
  stub: string
  commit: string - Git ref
  branch: string
  version: string - optional
  uncommitted: bool - optional, default false
body: key-number dictionary
logs: string
inputTokens: number
outputTokens: number
tokenCost: number
duration: number - seconds
```

## `abnch get scores` | `abnch get score`

Lists scores.

## `abnch get score <score ref>`

Outputs a score as JSON.
