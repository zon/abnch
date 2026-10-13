# CLI Design

## Credentials

Commands that reach the service authenticate a caller. The caller signs in with `abnch login`, which stores a session for the service URL. Later commands read the session and present it to the service. See the [Auth Design](auth.md).

Sessions live in a user-level `~/.abnch/credentials.yaml` file, keyed by service URL, outside any repo. A command that needs a session and finds none, or finds an expired one, fails and tells the user to run `abnch login`.

A service running locally trusts a dev issuer. `just config` writes a local CLI credential, which the CLI reads like a session. A runtime the CLI starts gets a Service token. `--local` chooses where agent or grader code runs, not how the caller authenticates. See the [Dev Config Design](dev-config.md).

## `abnch login`

Signs in a human user. The CLI runs the OIDC authorization code flow in a browser, receives the callback on a loopback address, and stores the resulting session for the service URL.

* --url: optional string - service url. Defaults to the [config](abnch-config.md) url

## `abnch logout`

Removes the stored session for the service URL.

* --url: optional string - service url. Defaults to the [config](abnch-config.md) url

## `set` Commands

`set` commands update if a matching resource already exists or create if the resource is missing. Updates are peformed by merging values passed to the command with the resource.

`set` commands raise an error if a required field is missing on both the resource and input.

## `abnch set config`

Create or updates an `.abnch/config.yaml` file in the repo under the working dir. See [CLI Config Design](abnch-config.md).

* --url: string - abnch service url

## `abnch get config`

Outputs the effective config as YAML.

## `abnch set model <provider>/<name>`

Create or updates model config. Adds a model option to the config file. See [CLI Config Design](abnch-config.md).

* --stub: optional string - override the auto generated stub
* --variant: optional string

## `abnch get models` | `abnch get model`

Lists models in the config.

## `abnch get model <stub>`

Outputs a model as YAML.

## `abnch set agent <task stub>`

Creates or updates agent config for a task. See [CLI Config Design](abnch-config.md).

* --version-cmd: string
* --build-cmd: string
* --run-cmd: string

## `abnch get agent <task stub>`

Outputs the task's agent config as YAML.

## `abnch set grader <task stub>`

Creates or updates grader config for a task. See [CLI Config Design](abnch-config.md).

* --version-cmd: string
* --build-cmd: string
* --run-cmd: string

## `abnch get grader <task stub>`

Outputs the task's grader config as YAML.

## `abnch set task <stub>`

Create or updates a task. Can recieve options or a YAML file with matching properties as stdin. The repo property is inferred from the working dir.

* --name: string - task name
* --description: string - task description
* --move: optional string - renames task from <stub> to --move value
* --model: optional string - sets the task's default model in config

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

## `abnch get data <task stub>/<test stub>/<ref>`

Outputs a datum as JSON.

## `abnch add data <task stub>/<test stub>`

Adds test data. Expects stdin or `--file` containing valid JSON.

* --file: relative path - a valid JSON file

## `abnch run <task stub>/<test stub>` | `abnch run <task stub>/<test stub>/<data ref>`

Run a test.

* --model: string - override default model
* --local: bool - run locally instead of using a workflow
* --limit: int - cap data iterations
* --shuffle: bool - randomly sort iterations

## `abnch score <result ref>`

Score a result.

* --model: string - override default model
* --local: bool - run locally instead of using a workflow

## `abnch get results` | `abnch get result`

Lists results.

## `abnch get result <result ref>`

Outputs a result as JSON.

## `abnch get scores` | `abnch get score`

Lists scores.

## `abnch get score <score ref>`

Outputs a score as JSON.
