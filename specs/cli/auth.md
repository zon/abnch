# Auth CLI Specification

## Purpose

Authenticate the `abnch` CLI to the service. Covers human sign-in and sign-out, the stored session, local credentials, and how commands present credentials. Authentication and roles are defined in the [Auth Service API](../service/auth.md). Authorization decisions belong to the service. Error output and exit codes follow the [Task CLI spec](task.md#requirement-cli-error-reporting).

## Requirements

### Requirement: Log In

`abnch login` SHALL sign in a human user through the OIDC provider and store a session for the service URL. Sign-in SHALL complete in a browser, without the user copying credentials.

On success the command SHALL exit zero. On failure it SHALL exit non-zero, write an error to stderr, and store no session.

#### Scenario: Successful login

- GIVEN the service URL is known
- WHEN the user runs `abnch login` and completes sign-in
- THEN the command exits zero
- AND a session for the service URL is stored

#### Scenario: Abandoned sign-in

- GIVEN the user starts `abnch login`
- WHEN the user does not complete sign-in
- THEN the command exits non-zero
- AND an error is written to stderr
- AND no session is stored

#### Scenario: Service without OIDC

- GIVEN the service has no OIDC provider
- WHEN the user runs `abnch login`
- THEN the command exits non-zero
- AND an error says OIDC login is unavailable
- AND no session is stored

### Requirement: Log Out

`abnch logout` SHALL remove the stored session for the service URL. Removing a session that is not stored SHALL succeed and change nothing.

#### Scenario: Log out

- GIVEN a stored session for the service URL
- WHEN the user runs `abnch logout`
- THEN the stored session is removed
- AND the command exits zero

#### Scenario: Log out with no session

- GIVEN no stored session for the service URL
- WHEN the user runs `abnch logout`
- THEN the command exits zero
- AND nothing changes

### Requirement: Service URL

The CLI SHALL take the service URL from `--url` when given, and from the [config](../../docs/design/app/abnch-config.md) otherwise. When neither provides one, `abnch login` and `abnch logout` SHALL fail.

#### Scenario: URL from config

- GIVEN the working directory has an `.abnch/config.yaml` with a url
- WHEN the user runs `abnch login`
- THEN the CLI signs in against that url

#### Scenario: URL override

- GIVEN the config has a url
- WHEN the user runs `abnch login --url https://bench.example.com`
- THEN the CLI signs in against `https://bench.example.com`

#### Scenario: No URL

- GIVEN no `--url` and no config url
- WHEN the user runs `abnch login`
- THEN the command exits non-zero
- AND an error is written to stderr

### Requirement: Session Storage

The CLI SHALL store the session in a user-level `~/.abnch/credentials.yaml` file, keyed by service URL. The session SHALL NOT be written into the repo.

A login or logout for one service URL SHALL NOT change the session for another.

#### Scenario: Session kept outside the repo

- GIVEN a repo with an `.abnch/config.yaml`
- WHEN the user logs in
- THEN the session is stored under the user home
- AND the repo files are unchanged

#### Scenario: Second service

- GIVEN a stored session for one service URL
- WHEN the user logs in to another service URL
- THEN both sessions are stored

### Requirement: Replace an Existing Session

Logging in again for the same service URL SHALL replace the stored session.

#### Scenario: Re-login

- GIVEN a stored session for the service URL
- WHEN the user logs in again
- THEN the session is replaced
- AND later commands use the new session

### Requirement: Session Reuse

A command that reaches a service without a local credential SHALL read the stored session for the service URL and present it. A command with no stored session, or an expired one, SHALL fail and tell the user to run `abnch login`.

#### Scenario: Command after login

- GIVEN a stored session for the service URL
- WHEN the user runs a command that reaches the service
- THEN the command presents the session
- AND succeeds when the service accepts it

#### Scenario: Expired session

- GIVEN the stored session for the service URL has expired
- WHEN the user runs a command that reaches the service
- THEN the command exits non-zero
- AND an error tells the user to run `abnch login`

#### Scenario: Command without a session

- GIVEN no stored session for the service URL
- WHEN the user runs a command that reaches the service
- THEN the command exits non-zero
- AND an error tells the user to run `abnch login`

### Requirement: Local Credentials

The CLI SHALL read a local credential that `just config` wrote for a local service and present it to that service. The credential SHALL live in the credential store, keyed by the local service URL.

A command that reaches a local service with a local credential SHALL NOT require `abnch login`.

#### Scenario: Local credential presented

- GIVEN `just config` wrote a local credential for the service URL
- WHEN the user runs a command that reaches the service
- THEN the CLI presents the local credential
- AND the command does not require a sign-in

#### Scenario: Configured by `just config`

- GIVEN `just config` wrote `.abnch/config.yaml` and the local credential for the local service
- WHEN the user runs a command that reaches the service
- THEN the CLI reads the service URL from `.abnch/config.yaml`
- AND it presents the local credential
- AND the command does not require a sign-in

### Requirement: Local Runtime Token

When the CLI starts a local runtime, it SHALL pass the runtime a workload token for the local service. The CLI SHALL pass the token in the runtime's environment.

#### Scenario: Runtime receives a token

- GIVEN the CLI starts a local runtime against a local service
- WHEN the runtime starts
- THEN the CLI passes it a workload token

### Requirement: Rejected Credentials

When the service rejects the caller with `401`, the CLI SHALL fail and tell the user to run `abnch login`. The CLI SHALL NOT retry the request with other credentials.

#### Scenario: Rejected session

- GIVEN a stored session the service rejects
- WHEN a command reaches the service
- THEN the command exits non-zero
- AND an error tells the user to run `abnch login`
- AND no request is retried with other credentials
