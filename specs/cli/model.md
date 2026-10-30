# Model CLI Specification

## Purpose

Read the models used by results and scores. The `add result` and `add score` commands record the model they use in the local [CLI config](../../docs/design/app/abnch-config.md) under a stub derived from the model's name. The `get` commands read them.

## Requirements

### Requirement: List Models

`abnch get models` SHALL list the models in the config as YAML.

#### Scenario: List models

- GIVEN the config records two models
- WHEN the user runs `abnch get models`
- THEN the output lists both models as YAML
- AND the command exits zero

#### Scenario: List models with no config

- GIVEN the working directory has no `.abnch/config.yaml`
- WHEN the user runs `abnch get models`
- THEN the output is empty
- AND the command exits zero

### Requirement: Get Model

`abnch get model <stub>` SHALL output one model as YAML. The command SHALL fail when the model is missing.

#### Scenario: Get an existing model

- GIVEN model `sonnet` exists in the config
- WHEN the user runs `abnch get model sonnet`
- THEN the model is written to stdout as YAML with its stub, name, provider, and variant
- AND the command exits zero

#### Scenario: Get a missing model

- GIVEN no model `sonnet` exists in the config
- WHEN the user runs `abnch get model sonnet`
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: CLI Error Reporting

The CLI SHALL exit zero on success and non-zero on failure. The CLI SHALL write error messages to stderr and command output to stdout.
