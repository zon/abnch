# Agent CLI Specification

## Purpose

Read the agent last used for a task. The `add result` command records the agent it uses in the local [CLI config](../../docs/design/app/abnch-config.md) under the result's task. The `get` command reads it.

## Requirements

### Requirement: Get Agent

`abnch get agent <task stub>` SHALL output the task's agent as YAML. The command SHALL fail when the task has no agent.

#### Scenario: Get an agent

- GIVEN task `fix-bug` has an agent with name `claude-code`, stub `claude`, commit `abc123`, branch `main`, version `1.2.3`, and uncommitted false
- WHEN the user runs `abnch get agent fix-bug`
- THEN the agent is written to stdout as YAML with its name, stub, commit, branch, version, and uncommitted flag
- AND the command exits zero

#### Scenario: Get a missing agent

- GIVEN task `fix-bug` has no agent
- WHEN the user runs `abnch get agent fix-bug`
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: CLI Error Reporting

The CLI SHALL exit zero on success and non-zero on failure. The CLI SHALL write error messages to stderr and command output to stdout.
