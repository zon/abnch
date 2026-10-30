# Result CLI Specification

## Purpose

Add a result for a test's datum and read results. A result links to the model and agent records that produced it. The result input is defined in the [Result Design](../../docs/design/result.md). The service contract is defined in the [Result Service API](../service/result.md).

## Requirements

### Requirement: Add a Result

`abnch add result <task>/<test>/<datum ref>` SHALL add a result for the datum and print its ref.

The command SHALL read a result input as YAML or JSON from stdin or `--file`. The input SHALL carry a `model` record, an `agent` record, and a `body` property holding the result. It MAY carry `logs`, `inputTokens`, `outputTokens`, `tokenCost`, and `duration`.

The model record SHALL carry a name, a provider, and a variant. The agent record SHALL carry a name, a stub, a commit, a branch, an optional version, and an uncommitted flag.

The CLI SHALL post the input to the service, which creates or reuses the model and agent records and links them to the result.

The command SHALL fail when the input carries no model, no agent, or no `body` property, when the datum is missing, or when the service rejects the write.

#### Scenario: Add a result

- GIVEN datum `<ref>` exists in `acme/widgets/fix-bug/basic`
- WHEN the user runs `abnch add result fix-bug/basic/<ref>` with a result input
- THEN the service creates or reuses the model and agent records
- AND the result is recorded with the model and agent records
- AND the command prints the result ref
- AND the command exits zero

#### Scenario: Missing model

- GIVEN a result input that carries no `model` record
- WHEN the user runs `abnch add result fix-bug/basic/<ref>`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no result is recorded

#### Scenario: Missing agent

- GIVEN a result input that carries no `agent` record
- WHEN the user runs `abnch add result fix-bug/basic/<ref>`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no result is recorded

#### Scenario: Missing result body

- GIVEN a result input that carries no `body` property
- WHEN the user runs `abnch add result fix-bug/basic/<ref>`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no result is recorded

#### Scenario: Missing datum

- GIVEN no datum with that ref exists
- WHEN the user runs `abnch add result fix-bug/basic/<ref>`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no result is recorded

### Requirement: Read Results

`abnch get results` SHALL list the inferred repo's results as YAML. `abnch get result <ref>` SHALL output one result as JSON with the model and agent records it links to. The command SHALL fail when the result is missing.

#### Scenario: List results

- GIVEN the inferred repo has two results
- WHEN the user runs `abnch get results`
- THEN the output lists both results as YAML
- AND the command exits zero

#### Scenario: Get a result

- GIVEN a result `<ref>` exists in the inferred repo
- WHEN the user runs `abnch get result <ref>`
- THEN the result is written to stdout as JSON
- AND the output includes the model and agent records
- AND the command exits zero

#### Scenario: Get a missing result

- GIVEN no result with that ref exists
- WHEN the user runs `abnch get result <ref>`
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: CLI Error Reporting

The CLI SHALL exit zero on success and non-zero on failure. The CLI SHALL write error messages to stderr and command output to stdout.

When the service returns an error, the CLI SHALL surface the service error message and fail.

#### Scenario: Service error

- GIVEN the service returns `403` for a result write
- WHEN the user runs `abnch add result fix-bug/basic/<ref>`
- THEN the command exits non-zero
- AND the service error message is written to stderr
