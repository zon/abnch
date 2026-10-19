# Config CLI Specification

## Purpose

Create, update, and read the local CLI config. The config at `.abnch/config.yaml` under the working directory records the service URL and the models, tasks, agents, and graders the CLI works with. Its schema is defined in the [CLI Config Design](../../docs/design/app/abnch-config.md).

## Requirements

### Requirement: Set Config

`abnch set config --url <url>` SHALL create or update the `url` in `.abnch/config.yaml` under the working directory. It SHALL create the `.abnch` directory when missing.

The command SHALL preserve every other key in the file. It SHALL fail when neither `--url` nor the existing config provides a URL.

#### Scenario: Create the config

- GIVEN the working directory has no `.abnch/config.yaml`
- WHEN the user runs `abnch set config --url https://bench.example.com`
- THEN `.abnch/config.yaml` holds `https://bench.example.com` as its url
- AND the command exits zero

#### Scenario: Update the URL

- GIVEN the config has a url and records a model
- WHEN the user runs `abnch set config --url https://new.example.com`
- THEN the config's url is `https://new.example.com`
- AND the model is unchanged

#### Scenario: No URL

- GIVEN the working directory has no `.abnch/config.yaml`
- WHEN the user runs `abnch set config`
- THEN the command exits non-zero
- AND an error naming `--url` is written to stderr
- AND no config is written

#### Scenario: Existing URL

- GIVEN the config has a url
- WHEN the user runs `abnch set config` without `--url`
- THEN the config's url is unchanged
- AND the command exits zero

### Requirement: Get Config

`abnch get config` SHALL write the config to stdout as YAML and exit zero. A missing config SHALL write nothing and exit zero.

#### Scenario: Read the config

- GIVEN the config has a url and records a model and a task
- WHEN the user runs `abnch get config`
- THEN the output is YAML carrying the url, the model, and the task
- AND the command exits zero

#### Scenario: Read with no config

- GIVEN the working directory has no `.abnch/config.yaml`
- WHEN the user runs `abnch get config`
- THEN the output is empty
- AND the command exits zero

### Requirement: Config Updates Preserve the Service URL

A `set` command that updates a model, task, agent, or grader SHALL preserve the config's url and every key the command does not name.

#### Scenario: Set a model

- GIVEN the config has a url
- WHEN the user runs `abnch set model anthropic/claude-sonnet`
- THEN the config still holds its url

#### Scenario: Set a task model

- GIVEN the config has a url
- WHEN the user runs `abnch set task fix-bug --model sonnet`
- THEN the config still holds its url

#### Scenario: Set an agent

- GIVEN the config has a url
- WHEN the user runs `abnch set agent fix-bug --run-cmd ./run.sh`
- THEN the config still holds its url

#### Scenario: Set a grader

- GIVEN the config has a url
- WHEN the user runs `abnch set grader fix-bug --run-cmd ./grade.sh`
- THEN the config still holds its url

### Requirement: Missing URL Message

A command that needs the service URL and finds none SHALL fail and name `abnch set config --url` as the way to set it.

#### Scenario: A command with no URL

- GIVEN the working directory has no config with a url
- WHEN the user runs a command that needs the service URL without `--url`
- THEN the command exits non-zero
- AND an error naming `abnch set config --url` is written to stderr
