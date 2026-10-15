# Data CLI Specification

## Purpose

Add and read a test's data with the `abnch` CLI. Data lives under a test in the repo the CLI derives from the working directory, as defined in the [Repo CLI spec](repo.md). The service contract is defined in the [Data Service API](../service/data.md).

## Requirements

### Requirement: Add Data

`abnch add data <task>/<test>` SHALL read a JSON value from stdin or from `--file` and add it to the test. The command SHALL print the datum's ref on success and exit zero.

The command SHALL fail when the input is not valid JSON or when the test is missing.

#### Scenario: Add from stdin

- GIVEN test `acme/widgets/fix-bug/basic` exists
- WHEN the user runs `abnch add data fix-bug/basic` with a JSON value on stdin
- THEN the datum's ref is written to stdout
- AND the command exits zero

#### Scenario: Add from a file

- GIVEN test `acme/widgets/fix-bug/basic` exists
- WHEN the user runs `abnch add data fix-bug/basic --file case.json` where the file holds valid JSON
- THEN the datum's ref is written to stdout
- AND the command exits zero

#### Scenario: Invalid JSON

- GIVEN the user provides input that is not valid JSON
- WHEN the user runs `abnch add data fix-bug/basic`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no datum is added

#### Scenario: Missing test

- GIVEN no test `acme/widgets/fix-bug/basic` exists
- WHEN the user runs `abnch add data fix-bug/basic` with a JSON value on stdin
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: List Data

`abnch get data <task>/<test>` SHALL list the test's data refs.

#### Scenario: List refs

- GIVEN test `acme/widgets/fix-bug/basic` has two data
- WHEN the user runs `abnch get data fix-bug/basic`
- THEN the output lists both refs
- AND the command exits zero

### Requirement: Get Data

`abnch get data <task>/<test>/<ref>` SHALL output one datum as JSON. The command SHALL fail when the datum is missing.

#### Scenario: Get an existing datum

- GIVEN datum `<ref>` exists in test `acme/widgets/fix-bug/basic`
- WHEN the user runs `abnch get data fix-bug/basic/<ref>`
- THEN the datum's JSON is written to stdout
- AND the command exits zero

#### Scenario: Get a missing datum

- GIVEN no datum with that ref exists
- WHEN the user runs `abnch get data fix-bug/basic/<ref>`
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: CLI Error Reporting

When the service returns an error, the CLI SHALL surface the service error message and fail.

#### Scenario: Service error

- GIVEN the service returns `403` for a data write
- WHEN the user runs `abnch add data fix-bug/basic`
- THEN the command exits non-zero
- AND the service error message is written to stderr
