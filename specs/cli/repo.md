# Repo CLI Specification

## Purpose

The `abnch` CLI derives the repo it acts on from the working directory. This spec covers that derivation. Managing repos directly is out of scope.

## Requirements

### Requirement: Infer Repo From the Working Directory

The CLI SHALL derive the repo's org stub and repo stub from the Git remote `origin` of the working directory.

The CLI SHALL support the common HTTPS and SSH remote forms, with or without a `.git` suffix or a trailing slash.

#### Scenario: HTTPS remote

- GIVEN the working directory has remote `origin` set to `https://github.com/acme/widgets.git`
- WHEN the CLI derives the repo
- THEN the org stub is `acme`
- AND the repo stub is `widgets`

#### Scenario: SSH remote

- GIVEN the working directory has remote `origin` set to `git@github.com:acme/widgets.git`
- WHEN the CLI derives the repo
- THEN the org stub is `acme`
- AND the repo stub is `widgets`

### Requirement: Ensure the Repo Exists

Before writing a task, the CLI SHALL ensure the inferred repo exists on the service. When it is missing, the CLI SHALL create it with the [Repo Service API](../service/repo.md) using the URL derived from the working directory.

#### Scenario: First task in a new repo

- GIVEN no repo `acme/widgets` exists on the service
- WHEN the user runs `abnch set task fix-bug`
- THEN the CLI creates `acme/widgets` on the service
- AND the task is written to that repo

#### Scenario: Existing repo is reused

- GIVEN repo `acme/widgets` exists on the service
- WHEN the user runs `abnch set task fix-bug`
- THEN the CLI does not create a repo
- AND the task is written to `acme/widgets`

### Requirement: Repo Inference Failure

When the CLI cannot derive a repo, commands that need one SHALL fail with a non-zero exit code and an error on stderr. The CLI SHALL NOT contact the service.

The CLI SHALL fail when the working directory is not a Git repo, has no remote `origin`, or has an origin the CLI cannot parse.

#### Scenario: Not a Git repo

- GIVEN the working directory is not a Git repo
- WHEN the user runs `abnch get tasks`
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no request is sent to the service

#### Scenario: No remote

- GIVEN the working directory is a Git repo with no remote `origin`
- WHEN the user runs `abnch get tasks`
- THEN the command exits non-zero
- AND an error is written to stderr

#### Scenario: Unparseable remote

- GIVEN remote `origin` is `https://github.com/acme` with no repo segment
- WHEN the user runs `abnch get tasks`
- THEN the command exits non-zero
- AND an error is written to stderr
