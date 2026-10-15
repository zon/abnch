# Grader CLI Specification

## Purpose

Record the grader for a task. A grader is external code that scores a result. The grader's commands run in the repo under the working directory. The grader's config lives in the local [CLI config](../../docs/design/app/abnch-config.md). Scoring a result is defined in the [Score CLI spec](score.md).

## Requirements

### Requirement: Set Grader

`abnch set grader <task stub>` SHALL create or update the task's grader in the config.

The command SHALL accept `--version-cmd`, `--build-cmd`, and `--run-cmd`, and SHALL merge them with an existing grader.

The command SHALL fail when the grader has no run command.

#### Scenario: Add a grader

- GIVEN the working directory has an `.abnch/config.yaml`
- WHEN the user runs `abnch set grader fix-bug --run-cmd ./grade.sh --build-cmd ./build.sh`
- THEN the config records a grader for `fix-bug` with that run command and build command
- AND the command exits zero

#### Scenario: Update a grader

- GIVEN task `fix-bug` has a grader with a run command
- WHEN the user runs `abnch set grader fix-bug --version-cmd ./version.sh`
- THEN the grader's version command is the new one
- AND the grader's run command is unchanged

#### Scenario: Missing run command

- GIVEN the task has no grader
- WHEN the user runs `abnch set grader fix-bug --build-cmd ./build.sh`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no grader is recorded

### Requirement: Get Grader

`abnch get grader <task stub>` SHALL output the task's grader as YAML. The command SHALL fail when the task has no grader.

#### Scenario: Get a grader

- GIVEN task `fix-bug` has a grader with a run command
- WHEN the user runs `abnch get grader fix-bug`
- THEN the grader is written to stdout as YAML with its version, build, and run commands
- AND the command exits zero

#### Scenario: Get a missing grader

- GIVEN task `fix-bug` has no grader
- WHEN the user runs `abnch get grader fix-bug`
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: CLI Error Reporting

The CLI SHALL exit zero on success and non-zero on failure. The CLI SHALL write error messages to stderr and command output to stdout.
