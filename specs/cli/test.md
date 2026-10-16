# Test CLI Specification

## Purpose

Create, update, and read tests with the `abnch` CLI. A test lives under a task in the repo the CLI derives from the working directory, as defined in the [Repo CLI spec](repo.md). The service contract is defined in the [Test Service API](../service/test.md).

## Requirements

### Requirement: Set Test

`abnch set test <task>/<stub>` SHALL create a test when it is missing and update it when it exists.

The command SHALL accept `--name`, `--description`, and `--test-version` options, and SHALL also accept a YAML or JSON document on stdin with matching properties, including the score schema. Values from options and stdin SHALL merge with the existing test.

The command SHALL fail when a required field is missing from both the input and the existing test, and when the task is missing.

#### Scenario: Create a test

- GIVEN the working directory derives repo `acme/widgets`
- AND task `fix-bug` exists and no test `basic` exists
- WHEN the user runs `abnch set test fix-bug/basic --name "Basic" --test-version 1`
- THEN the test is created in `acme/widgets/fix-bug`
- AND `abnch get test fix-bug/basic` returns it

#### Scenario: Set the schema from stdin

- GIVEN test `acme/widgets/fix-bug/basic` exists
- WHEN the user runs `abnch set test fix-bug/basic` with a YAML body carrying a schema
- THEN the schema is replaced
- AND the name and version are unchanged

#### Scenario: Missing required field

- GIVEN no test `basic` exists
- WHEN the user runs `abnch set test fix-bug/basic` with only a description
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no test is created

#### Scenario: Missing task

- GIVEN no task `fix-bug` exists
- WHEN the user runs `abnch set test fix-bug/basic --name "Basic" --test-version 1`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no task or test is created

### Requirement: Rename Test

`abnch set test <task>/<stub> --move <new>` SHALL rename the test from `<stub>` to `<new>`.

The command SHALL fail when the test is missing or when `<new>` is already taken.

#### Scenario: Rename a test

- GIVEN test `acme/widgets/fix-bug/basic` exists and `smoke` is free
- WHEN the user runs `abnch set test fix-bug/basic --move smoke`
- THEN the test is renamed
- AND `abnch get test fix-bug/smoke` returns it

#### Scenario: Rename onto an existing stub

- GIVEN tests `basic` and `smoke` exist in `acme/widgets/fix-bug`
- WHEN the user runs `abnch set test fix-bug/basic --move smoke`
- THEN the command exits non-zero
- AND both tests are unchanged

### Requirement: Get Tests

`abnch get tests <task>` SHALL list the task's tests as YAML.

#### Scenario: List tests

- GIVEN task `acme/widgets/fix-bug` has two tests
- WHEN the user runs `abnch get tests fix-bug`
- THEN the output lists both tests as YAML
- AND the command exits zero

### Requirement: Get Test

`abnch get test <task>/<stub>` SHALL output one test as YAML. The command SHALL fail when the test is missing.

#### Scenario: Get an existing test

- GIVEN test `acme/widgets/fix-bug/basic` exists
- WHEN the user runs `abnch get test fix-bug/basic`
- THEN the test is written to stdout as YAML with its stub, name, description, version, and schema
- AND the command exits zero

#### Scenario: Get a missing test

- GIVEN no test `basic` exists
- WHEN the user runs `abnch get test fix-bug/basic`
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: CLI Error Reporting

The CLI SHALL exit zero on success and non-zero on failure. The CLI SHALL write error messages to stderr and command output to stdout.

When the service returns an error, the CLI SHALL surface the service error message and fail.

#### Scenario: Service error

- GIVEN the service returns `403` for a test write
- WHEN the user runs `abnch set test fix-bug/basic --name "Basic" --test-version 1`
- THEN the command exits non-zero
- AND the service error message is written to stderr
