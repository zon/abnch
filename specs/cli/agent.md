# Agent CLI Specification

## Purpose

Record the agent for a task. An agent is external code that produces a result from a datum. The agent's commands run in the repo under the working directory. The agent's config lives in the local [CLI config](../../docs/design/app/abnch-config.md). Running an agent is defined in the [Run CLI spec](run.md).

## Requirements

### Requirement: Set Agent

`abnch set agent <task stub>` SHALL create or update the task's agent in the config.

The command SHALL accept `--version-cmd`, `--build-cmd`, and `--run-cmd`, and SHALL merge them with an existing agent.

The command SHALL fail when the agent has no run command.

#### Scenario: Add an agent

- GIVEN the working directory has an `.abnch/config.yaml`
- WHEN the user runs `abnch set agent fix-bug --run-cmd ./run.sh --build-cmd ./build.sh`
- THEN the config records an agent for `fix-bug` with that run command and build command
- AND the command exits zero

#### Scenario: Update an agent

- GIVEN task `fix-bug` has an agent with a run command
- WHEN the user runs `abnch set agent fix-bug --version-cmd ./version.sh`
- THEN the agent's version command is the new one
- AND the agent's run command is unchanged

#### Scenario: Missing run command

- GIVEN the task has no agent
- WHEN the user runs `abnch set agent fix-bug --build-cmd ./build.sh`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no agent is recorded

### Requirement: Get Agent

`abnch get agent <task stub>` SHALL output the task's agent as YAML. The command SHALL fail when the task has no agent.

#### Scenario: Get an agent

- GIVEN task `fix-bug` has an agent with a run command
- WHEN the user runs `abnch get agent fix-bug`
- THEN the agent is written to stdout as YAML with its version, build, and run commands
- AND the command exits zero

#### Scenario: Get a missing agent

- GIVEN task `fix-bug` has no agent
- WHEN the user runs `abnch get agent fix-bug`
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: CLI Error Reporting

The CLI SHALL exit zero on success and non-zero on failure. The CLI SHALL write error messages to stderr and command output to stdout.
