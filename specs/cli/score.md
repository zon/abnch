# Score CLI Specification

## Purpose

Score a result with a task's grader and read scores. A grade run executes locally. Workflow execution is out of scope. The grader config is defined in the [Grader CLI spec](grader.md). The model config is defined in the [Model CLI spec](model.md). The service contract is defined in the [Score Service API](../service/score.md).

## Requirements

### Requirement: Score a Result

`abnch score <result ref>` SHALL run the task's grader against the result and record a score.

The CLI SHALL read the result from the service, read the task's grader from the config, resolve the model, build the grader once, run it against the result, and record the score.

The command SHALL accept `--model` and `--local`. `--model` overrides the task's default model. `--local` selects a local run, which is the only mode.

The command SHALL fail when the task has no grader, when no model is set, or when the result is missing.

#### Scenario: Score a result

- GIVEN result `<ref>` exists in the inferred repo
- AND the task has a grader and a model
- WHEN the user runs `abnch score <ref>`
- THEN the grader runs against the result
- AND a score is recorded for the result
- AND the command exits zero

#### Scenario: Missing grader

- GIVEN the task has no grader
- WHEN the user runs `abnch score <ref>`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND the grader does not run

#### Scenario: Missing model

- GIVEN the task has no default model and no `--model` is given
- WHEN the user runs `abnch score <ref>`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND the grader does not run

#### Scenario: Unknown model

- GIVEN the config holds no model `ghost`
- WHEN the user runs `abnch score <ref> --model ghost`
- THEN the command exits non-zero
- AND an error is written to stderr

#### Scenario: Missing result

- GIVEN no result with that ref exists
- WHEN the user runs `abnch score <ref>`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND the grader does not run

### Requirement: Grader Input

The CLI SHALL pass a JSON object to the grader's run command on stdin.

The object SHALL carry the model's name, provider, and variant, the JSON of the datum that produced the result, and the result's JSON.

#### Scenario: Grader reads the model, the datum, and the result

- GIVEN a model, a datum, and its result
- WHEN the grader's run command runs
- THEN stdin carries the model's name, provider, and variant, the datum's JSON, and the result's JSON

### Requirement: Score Output

The grader's run command SHALL write its logs to stdout, then the abnch marker, then a score output as JSON. The marker and the score output are defined in the [Score Design](../../docs/design/score.md).

The CLI SHALL treat stdout before the marker as the score's logs and parse the JSON after the marker as the score output. The score output SHALL carry a `score` property holding the key-number dictionary. It MAY carry `inputTokens`, `outputTokens`, and `tokenCost`.

The CLI SHALL measure the run's duration and record it with the score.

A grade run whose stdout carries no marker, or whose score output carries no `score` property, SHALL fail.

#### Scenario: Record the score output

- GIVEN a grader that writes logs, the marker, and a score output with a dictionary and token counts
- WHEN the grader runs
- THEN the score carries the logs, the dictionary, the token counts, and the run duration

#### Scenario: Missing marker

- GIVEN a grader that writes no marker
- WHEN the user runs `abnch score <ref>`
- THEN the command exits non-zero
- AND an error is written to stderr

#### Scenario: Missing score

- GIVEN a grader whose score output carries no `score` property
- WHEN the user runs `abnch score <ref>`
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: Read Scores

`abnch get scores` SHALL list the inferred repo's scores as YAML. `abnch get score <ref>` SHALL output one score as JSON. The command SHALL fail when the score is missing.

#### Scenario: List scores

- GIVEN the inferred repo has two scores
- WHEN the user runs `abnch get scores`
- THEN the output lists both scores as YAML
- AND the command exits zero

#### Scenario: Get a score

- GIVEN a score `<ref>` exists in the inferred repo
- WHEN the user runs `abnch get score <ref>`
- THEN the score is written to stdout as JSON
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
- WHEN the user runs `abnch score <ref>`
- THEN the command exits non-zero
- AND the service error message is written to stderr
