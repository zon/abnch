# Score CLI Specification

## Purpose

Add a score for a result and read scores. A score links to the model and grader records that produced it. The score input is defined in the [Score Design](../../docs/design/score.md). The service contract is defined in the [Score Service API](../service/score.md).

## Requirements

### Requirement: Add a Score

`abnch add score <result ref>` SHALL add a score for the result and print its ref.

The command SHALL read a score input as YAML or JSON from stdin or `--file`. The input SHALL carry a `model` record, a `grader` record, and a `dictionary` property holding the key-number dictionary. It MAY carry `logs`, `inputTokens`, `outputTokens`, `tokenCost`, and `duration`.

The model record SHALL carry a name, a provider, and a variant. The grader record SHALL carry a name, a stub, a commit, a branch, an optional version, and an uncommitted flag.

The CLI SHALL read the result to find its task and test. It SHALL post the input to the service, which creates or reuses the model and grader records and links them to the score. The CLI SHALL record the model and grader in the config, so `abnch get model` and `abnch get grader` can read them.

The command SHALL fail when the result is missing, when the input carries no model, no grader, or no `dictionary`, or when the service rejects the write.

#### Scenario: Add a score

- GIVEN result `<ref>` exists in the inferred repo
- WHEN the user runs `abnch add score <ref>` with a score input
- THEN the service creates or reuses the model and grader records
- AND the score is recorded with the model and grader records
- AND the CLI records the model and grader in the config
- AND the command prints the score ref
- AND the command exits zero

#### Scenario: Missing model

- GIVEN a score input that carries no `model` record
- WHEN the user runs `abnch add score <ref>`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no score is recorded

#### Scenario: Missing grader

- GIVEN a score input that carries no `grader` record
- WHEN the user runs `abnch add score <ref>`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no score is recorded

#### Scenario: Missing dictionary

- GIVEN a score input that carries no `dictionary` property
- WHEN the user runs `abnch add score <ref>`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no score is recorded

#### Scenario: Missing result

- GIVEN no result with that ref exists
- WHEN the user runs `abnch add score <ref>`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no score is recorded

### Requirement: Read Scores

`abnch get scores` SHALL list the inferred repo's scores as YAML. `abnch get score <ref>` SHALL output one score as JSON with the model and grader records it links to. The command SHALL fail when the score is missing.

#### Scenario: List scores

- GIVEN the inferred repo has two scores
- WHEN the user runs `abnch get scores`
- THEN the output lists both scores as YAML
- AND the command exits zero

#### Scenario: Get a score

- GIVEN a score `<ref>` exists in the inferred repo
- WHEN the user runs `abnch get score <ref>`
- THEN the score is written to stdout as JSON
- AND the output includes the model and grader records
- AND the command exits zero

#### Scenario: Get a missing score

- GIVEN no score with that ref exists
- WHEN the user runs `abnch get score <ref>`
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: CLI Error Reporting

The CLI SHALL exit zero on success and non-zero on failure. The CLI SHALL write error messages to stderr and command output to stdout.

When the service returns an error, the CLI SHALL surface the service error message and fail.

#### Scenario: Service error

- GIVEN the service returns `403` for a score write
- WHEN the user runs `abnch add score <ref>`
- THEN the command exits non-zero
- AND the service error message is written to stderr
