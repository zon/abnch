# Grader CLI Specification

## Purpose

Read the grader last used for a task. The `add score` command records the grader it uses in the local [CLI config](../../docs/design/app/abnch-config.md) under the result's task. The `get` command reads it.

## Requirements

### Requirement: Get Grader

`abnch get grader <task stub>` SHALL output the task's grader as YAML. The command SHALL fail when the task has no grader.

#### Scenario: Get a grader

- GIVEN task `fix-bug` has a grader with name `llm-judge`, stub `judge`, commit `def456`, branch `main`, version `2.0.0`, and uncommitted false
- WHEN the user runs `abnch get grader fix-bug`
- THEN the grader is written to stdout as YAML with its name, stub, commit, branch, version, and uncommitted flag
- AND the command exits zero

#### Scenario: Get a missing grader

- GIVEN task `fix-bug` has no grader
- WHEN the user runs `abnch get grader fix-bug`
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: CLI Error Reporting

The CLI SHALL exit zero on success and non-zero on failure. The CLI SHALL write error messages to stderr and command output to stdout.
