# Result Service API Specification

## Purpose

Store and read the results an agent produces for a test's data. A result belongs to a datum. Error shapes and authorization follow the [Repo Service API](repo.md). Scoring a result is out of scope.

## Requirements

### Requirement: Result Identity

A result SHALL be identified by its datum and a ref. The ref SHALL be a short, URL-safe string derived from the result JSON within its datum.

Two results with the same JSON for the same datum SHALL have the same ref and SHALL be the same result. Results for different data SHALL have different identities even when their JSON matches.

#### Scenario: Same JSON, same ref

- GIVEN a result with JSON `{"answer": "hi"}` exists for `acme/widgets/fix-bug/basic/<data>`
- WHEN an authorized client adds the same JSON for the same datum
- THEN the service reports the same ref
- AND the datum still holds one result

#### Scenario: Same JSON, different data

- GIVEN a result exists for one datum in a test
- WHEN an authorized client adds the same JSON for another datum in the same test
- THEN the service reports a different ref

### Requirement: Result Fields

A result SHALL carry its JSON, logs, input tokens, output tokens, token cost, and run duration. It MAY carry the local branch that produced it.

Token counts, token cost, and run duration SHALL be numbers that are zero or greater. A result that omits them SHALL record zero.

#### Scenario: Result carries its fields

- GIVEN a result exists with a JSON value, logs, token counts, a token cost, and a run duration
- WHEN an authorized client gets it
- THEN the body includes the JSON, the logs, the tokens, the cost, and the duration

#### Scenario: Fields default to zero

- GIVEN an authorized client adds a result without token counts, a cost, or a duration
- WHEN the service stores it
- THEN the token counts, the cost, and the duration are zero

#### Scenario: Negative count

- GIVEN an authorized client adds a result with a negative token count
- WHEN the service handles it
- THEN the status is `422`
- AND no result is created

### Requirement: Result Parent

A result SHALL belong to a datum. Adding a result to a missing datum SHALL be an error, and the service SHALL NOT create the datum or the result.

#### Scenario: Add a result to a missing datum

- GIVEN no datum `<ref>` exists in `acme/widgets/fix-bug/basic`
- WHEN an authorized client adds a result for it
- THEN the status is `404`
- AND no datum or result is created

### Requirement: Add Result

`POST /repos/{org}/{repo}/results` SHALL add a result from the request body and return its ref.

The body SHALL carry the task stub, the test stub, the datum ref, and the result JSON. It MAY carry logs, input tokens, output tokens, token cost, run duration, and the local branch.

The service SHALL derive the ref from the datum and the JSON, so adding the same JSON for the same datum again returns the existing ref and creates no second result.

The service SHALL return `201` for a new ref and `200` when the ref already exists. A malformed body SHALL return `422`. A missing datum SHALL return `404`.

#### Scenario: Add a result

- GIVEN a datum `acme/widgets/fix-bug/basic/<data>` exists
- WHEN an authorized client sends `POST /repos/acme/widgets/results` with the task, the test, the datum ref, and a JSON body
- THEN the status is `201`
- AND the body carries the result's ref
- AND a later `GET` returns the result

#### Scenario: Add the same result twice

- GIVEN a result with JSON `{"answer": "hi"}` exists for a datum
- WHEN an authorized client posts the same JSON for the same datum
- THEN the status is `200`
- AND the service reports the existing ref
- AND the datum holds one result

#### Scenario: Malformed body

- GIVEN a datum exists
- WHEN an authorized client sends `POST /repos/acme/widgets/results` with a body that carries no JSON
- THEN the status is `422`
- AND no result is created

#### Scenario: Missing datum

- GIVEN no datum `<ref>` exists
- WHEN an authorized client sends `POST /repos/acme/widgets/results` for it
- THEN the status is `404`
- AND no result is created

### Requirement: List Results

`GET /repos/{org}/{repo}/results` SHALL return the repo's result refs as a JSON array.

#### Scenario: List results

- GIVEN two results exist in `acme/widgets`
- WHEN an authorized client sends `GET /repos/acme/widgets/results`
- THEN the status is `200`
- AND the body lists both refs

#### Scenario: List results in a missing repo

- GIVEN no repo `acme/widgets` exists
- WHEN an authorized client sends `GET /repos/acme/widgets/results`
- THEN the status is `404`

### Requirement: Get Result

`GET /repos/{org}/{repo}/results/{ref}` SHALL return one result as JSON with its ref, task, test, datum ref, JSON, logs, tokens, cost, duration, and local branch.

#### Scenario: Get an existing result

- GIVEN a result `<ref>` exists in `acme/widgets`
- WHEN an authorized client sends `GET /repos/acme/widgets/results/<ref>`
- THEN the status is `200`
- AND the body includes the ref, the task, the test, the datum ref, the JSON, the logs, the tokens, the cost, the duration, and the local branch

#### Scenario: Get a missing result

- GIVEN no result with that ref exists
- WHEN an authorized client sends `GET /repos/acme/widgets/results/<ref>`
- THEN the status is `404`

### Requirement: Authorization

Read operations SHALL be allowed for the Reader, Service, and Admin roles. Write operations SHALL be allowed for the Service and Admin roles.

The service SHALL return `401` for an unauthenticated request and `403` for a role without access.

#### Scenario: Reader reads a result

- GIVEN a caller with the Reader role
- WHEN the caller sends `GET /repos/acme/widgets/results`
- THEN the status is `200`

#### Scenario: Service adds a result

- GIVEN a caller with the Service role
- WHEN the caller sends `POST /repos/acme/widgets/results` with a valid body
- THEN the status is `2xx`

#### Scenario: Reader adds a result

- GIVEN a caller with the Reader role
- WHEN the caller sends `POST /repos/acme/widgets/results` with a valid body
- THEN the status is `403`
- AND no result is created
