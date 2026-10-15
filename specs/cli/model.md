# Model CLI Specification

## Purpose

Record the models a task's agent may call. Models live in the local [CLI config](../../docs/design/app/abnch-config.md) and are not stored on the service. A run passes a model to the agent, as defined in the [Run CLI spec](run.md).

## Requirements

### Requirement: Set Model

`abnch set model <provider>/<name>` SHALL create or update a model in the config.

The command SHALL record the model's provider and name. It SHALL accept `--variant` and SHALL store it when given. It SHALL accept `--stub`, and SHALL derive the stub from the name otherwise.

#### Scenario: Add a model

- GIVEN the working directory has an `.abnch/config.yaml`
- WHEN the user runs `abnch set model anthropic/claude-sonnet --variant high`
- THEN the config records a model with provider `anthropic`, name `claude-sonnet`, and variant `high`
- AND the command exits zero

#### Scenario: Choose the stub

- GIVEN the working directory has an `.abnch/config.yaml`
- WHEN the user runs `abnch set model anthropic/claude-sonnet --stub sonnet`
- THEN the model is recorded under `sonnet`

#### Scenario: Update a model

- GIVEN model `sonnet` exists in the config
- WHEN the user runs `abnch set model anthropic/claude-sonnet --stub sonnet --variant low`
- THEN the model's variant is `low`
- AND the model's name and provider are unchanged

#### Scenario: Missing provider

- GIVEN the user provides `claude-sonnet` with no provider
- WHEN the user runs `abnch set model claude-sonnet`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no model is recorded

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
- THEN the model is written to stdout as YAML with its stub, provider, name, and variant
- AND the command exits zero

#### Scenario: Get a missing model

- GIVEN no model `sonnet` exists in the config
- WHEN the user runs `abnch get model sonnet`
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: CLI Error Reporting

The CLI SHALL exit zero on success and non-zero on failure. The CLI SHALL write error messages to stderr and command output to stdout.
