# Repo Service API Specification

## Purpose

Manage repos over the service API. A repo groups tasks. This spec also defines the API conventions that other service resources share. Deleting repos is out of scope.

## Requirements

### Requirement: API Conventions

The service SHALL expose resources over HTTP and exchange JSON bodies.

Successful responses SHALL use a `2xx` status and a JSON body. Error responses SHALL use a `4xx` or `5xx` status and a body shaped `{ "error": { "message": string } }`.

Routes SHALL be relative to the service URL in the [config](../../docs/design/cli-config.md).

#### Scenario: Error body

- GIVEN a request for a repo that does not exist
- WHEN the service responds
- THEN the status is `404`
- AND the body is `{ "error": { "message": "..." } }`

### Requirement: Repo Identity

A repo SHALL be identified by an org stub and a repo stub. Both SHALL be non-empty, URL-safe strings.

Two repos with the same org stub and repo stub SHALL be the same repo, regardless of URL.

#### Scenario: Same identity, different URL

- GIVEN a repo `acme/widgets` with URL `https://github.com/acme/widgets`
- WHEN a client updates `acme/widgets` with a new URL
- THEN the update applies to the same repo
- AND no second repo is created

### Requirement: List Repos

`GET /repos` SHALL return the repos visible to the caller as a JSON array.

#### Scenario: List repos

- GIVEN two repos exist
- WHEN an authorized client sends `GET /repos`
- THEN the status is `200`
- AND the body lists both repos

### Requirement: Get Repo

`GET /repos/{org}/{stub}` SHALL return one repo as JSON with its org stub, repo stub, and URL.

#### Scenario: Get an existing repo

- GIVEN a repo `acme/widgets` exists
- WHEN an authorized client sends `GET /repos/acme/widgets`
- THEN the status is `200`
- AND the body includes the org stub, repo stub, and URL

#### Scenario: Get a missing repo

- GIVEN no repo `acme/widgets` exists
- WHEN an authorized client sends `GET /repos/acme/widgets`
- THEN the status is `404`

### Requirement: Create or Update Repo

`PUT /repos/{org}/{stub}` SHALL create the repo when it is missing and update it when it exists.

The body SHALL carry the repo URL. On update, the service SHALL merge the body into the existing repo. A missing URL SHALL be an error.

The service SHALL return `201` on create and `200` on update.

#### Scenario: Create a repo

- GIVEN no repo `acme/widgets` exists
- WHEN an authorized client sends `PUT /repos/acme/widgets` with a URL
- THEN the status is `201`
- AND a later `GET` returns the repo

#### Scenario: Update a repo

- GIVEN a repo `acme/widgets` exists
- WHEN an authorized client sends `PUT /repos/acme/widgets` with a new URL
- THEN the status is `200`
- AND the URL is replaced

#### Scenario: Missing URL on create

- GIVEN no repo `acme/widgets` exists
- WHEN an authorized client sends `PUT /repos/acme/widgets` without a URL
- THEN the status is `422`
- AND no repo is created

### Requirement: Invalid Identity

The service SHALL reject a repo whose org stub or repo stub is empty or not URL-safe.

#### Scenario: Empty stub

- WHEN an authorized client sends `PUT /repos/acme/`
- THEN the status is `404`

#### Scenario: Unsafe stub

- WHEN an authorized client sends `PUT /repos/acme/a%2Fb` where the stub contains a separator
- THEN the status is `422`
- AND no repo is created

### Requirement: Authorization

Read operations SHALL be allowed for the Reader, Service, and Admin roles. Write operations SHALL be allowed for the Admin role only.

The service SHALL return `401` for an unauthenticated request and `403` for a role without access. Roles are defined in the [Auth Design](../../docs/design/auth.md).

#### Scenario: Reader lists repos

- GIVEN a caller with the Reader role
- WHEN the caller sends `GET /repos`
- THEN the status is `200`

#### Scenario: Reader writes a repo

- GIVEN a caller with the Reader role
- WHEN the caller sends `PUT /repos/acme/widgets`
- THEN the status is `403`
- AND no repo is created

#### Scenario: Unauthenticated request

- GIVEN a request with no credentials
- WHEN the service receives it
- THEN the status is `401`
