# Task CLI Specification

## Purpose

Create, update, and read tasks with the `abnch` CLI. Tasks are scoped to the repo the CLI derives from the working directory, as defined in the [Repo CLI spec](repo.md). The service contract is defined in the [Task Service API](../service/task.md).

## Requirements

### Requirement: Set Task

`abnch set task <stub>` SHALL create a task when it is missing and update it when it exists. The repo SHALL be inferred from the working directory.

The command SHALL accept `--name` and `--description` options, and SHALL also accept a YAML or JSON document on stdin with matching properties. Values from options and stdin SHALL merge with the existing task.

The command SHALL fail when a required field is missing from both the input and the existing task.

#### Scenario: Create a task

- GIVEN the working directory derives repo `acme/widgets`
- AND no task `fix-bug` exists
- WHEN the user runs `abnch set task fix-bug --name "Fix bug" --description "Fix the bug"`
- THEN the task is created in `acme/widgets`
- AND `abnch get task fix-bug` returns it

#### Scenario: Update from stdin

- GIVEN task `acme/widgets/fix-bug` exists with a name
- WHEN the user runs `abnch set task fix-bug` with a YAML body containing a new description
- THEN the description is updated
- AND the name is unchanged

#### Scenario: Missing required field

- GIVEN no task `fix-bug` exists
- WHEN the user runs `abnch set task fix-bug` with only a description
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no task is created

### Requirement: Rename Task

`abnch set task <stub> --move <new>` SHALL rename the task from `<stub>` to `<new>`.

The command SHALL fail when the task is missing or when `<new>` is already taken.

#### Scenario: Rename a task

- GIVEN task `acme/widgets/fix-bug` exists and `fix-typo` is free
- WHEN the user runs `abnch set task fix-bug --move fix-typo`
- THEN the task is renamed
- AND `abnch get task fix-typo` returns it

#### Scenario: Rename onto an existing stub

- GIVEN tasks `fix-bug` and `fix-typo` exist
- WHEN the user runs `abnch set task fix-bug --move fix-typo`
- THEN the command exits non-zero
- AND both tasks are unchanged

### Requirement: Default Model

`abnch set task <stub> --model <model stub>` SHALL set the task's default model in the local [config](../../docs/design/cli-config.md). It SHALL NOT change the task on the service.

#### Scenario: Set the default model

- GIVEN the working directory has an `.abnch/config.yaml`
- WHEN the user runs `abnch set task fix-bug --model sonnet`
- THEN `sonnet` is recorded as the task's model in the config
- AND the task on the service is unchanged

### Requirement: Get Tasks

`abnch get tasks` SHALL list the tasks in the inferred repo as YAML.

#### Scenario: List tasks

- GIVEN repo `acme/widgets` has two tasks
- WHEN the user runs `abnch get tasks`
- THEN the output lists both tasks as YAML
- AND the command exits zero

### Requirement: Get Task

`abnch get task <stub>` SHALL output one task as YAML. The command SHALL fail when the task is missing.

#### Scenario: Get an existing task

- GIVEN task `acme/widgets/fix-bug` exists
- WHEN the user runs `abnch get task fix-bug`
- THEN the task is written to stdout as YAML
- AND the command exits zero

#### Scenario: Get a missing task

- GIVEN no task `fix-bug` exists
- WHEN the user runs `abnch get task fix-bug`
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: CLI Error Reporting

The CLI SHALL exit zero on success and non-zero on failure. The CLI SHALL write error messages to stderr and command output to stdout.

When the service returns an error, the CLI SHALL surface the service error message and fail.

#### Scenario: Service error

- GIVEN the service returns `403` for a task write
- WHEN the user runs `abnch set task fix-bug`
- THEN the command exits non-zero
- AND the service error message is written to stderr
