# Data Service API Specification

## Purpose

Manage a test's data over the service API. A datum is the JSON a test passes to an agent, one per case. Error shapes and authorization follow the [Repo Service API](repo.md). Removing data is out of scope.

## Requirements

### Requirement: Data Identity

A datum SHALL be identified by its repo, task stub, test stub, and ref. The ref SHALL be a short, URL-safe string derived from the datum's JSON.

Two data with the same JSON in the same test SHALL have the same ref and SHALL be the same datum. Two data with different JSON SHALL have different refs.

#### Scenario: Same JSON, same ref

- GIVEN a datum with JSON `{"prompt": "hi"}` is in `acme/widgets/fix-bug/basic`
- WHEN an authorized client adds the same JSON
- THEN the service reports the same ref
- AND the test still holds one datum

#### Scenario: Different JSON, different ref

- GIVEN a test holds a datum with JSON `{"prompt": "hi"}`
- WHEN an authorized client adds a datum with JSON `{"prompt": "bye"}`
- THEN the service reports a different ref

### Requirement: Data Value

A datum SHALL carry its JSON unchanged. The service SHALL store and return the JSON as the client gave it.

#### Scenario: JSON round trips

- GIVEN a datum is added with a JSON value
- WHEN an authorized client gets it
- THEN the body is the same JSON value

### Requirement: File References

A datum's JSON MAY reference a file. A file reference SHALL be a JSON object carrying a `url` and a `checksum`.

The service SHALL store a file reference unchanged and SHALL NOT fetch the file. Agents and graders SHALL download and verify the file.

#### Scenario: File reference round trips

- GIVEN a datum whose JSON holds `{ "url": "https://example.com/input.txt", "checksum": "sha256:..." }`
- WHEN an authorized client gets the datum
- THEN the file reference is unchanged

### Requirement: List Data

`GET /repos/{org}/{repo}/tasks/{task}/tests/{test}/data` SHALL return the test's data refs as a JSON array.

#### Scenario: List data

- GIVEN two data exist in `acme/widgets/fix-bug/basic`
- WHEN an authorized client sends `GET /repos/acme/widgets/tasks/fix-bug/tests/basic/data`
- THEN the status is `200`
- AND the body lists both refs

#### Scenario: List data in a missing test

- GIVEN no test `acme/widgets/fix-bug/basic` exists
- WHEN an authorized client sends `GET /repos/acme/widgets/tasks/fix-bug/tests/basic/data`
- THEN the status is `404`

### Requirement: Get Data

`GET /repos/{org}/{repo}/tasks/{task}/tests/{test}/data/{ref}` SHALL return one datum as JSON.

#### Scenario: Get an existing datum

- GIVEN a datum `acme/widgets/fix-bug/basic/<ref>` exists
- WHEN an authorized client sends `GET /repos/acme/widgets/tasks/fix-bug/tests/basic/data/<ref>`
- THEN the status is `200`
- AND the body is the datum's JSON

#### Scenario: Get a missing datum

- GIVEN no datum with that ref exists in the test
- WHEN an authorized client sends `GET /repos/acme/widgets/tasks/fix-bug/tests/basic/data/<ref>`
- THEN the status is `404`

### Requirement: Add Data

`POST /repos/{org}/{repo}/tasks/{task}/tests/{test}/data` SHALL add a datum from the request body and return its ref.

The body SHALL be valid JSON. The service SHALL derive the ref from the JSON, so adding the same JSON again returns the existing ref and creates no second datum.

The service SHALL return `201` for a new ref and `200` when the ref already exists. A malformed body SHALL return `422`. A missing test SHALL return `404`.

#### Scenario: Add a datum

- GIVEN a test `acme/widgets/fix-bug/basic` exists
- WHEN an authorized client sends `POST /repos/acme/widgets/tasks/fix-bug/tests/basic/data` with a JSON body
- THEN the status is `201`
- AND the body carries the datum's ref
- AND a later `GET` returns the datum

#### Scenario: Add the same JSON twice

- GIVEN a datum with JSON `{"prompt": "hi"}` exists in a test
- WHEN an authorized client posts the same JSON
- THEN the status is `200`
- AND the service reports the existing ref
- AND the test holds one datum

#### Scenario: Malformed body

- GIVEN a test exists
- WHEN an authorized client sends `POST /repos/acme/widgets/tasks/fix-bug/tests/basic/data` with a body that is not valid JSON
- THEN the status is `422`
- AND no datum is created

#### Scenario: Add data to a missing test

- GIVEN no test `acme/widgets/fix-bug/basic` exists
- WHEN an authorized client sends `POST /repos/acme/widgets/tasks/fix-bug/tests/basic/data` with a JSON body
- THEN the status is `404`
- AND no datum is created

### Requirement: Authorization

Read operations SHALL be allowed for the Reader, Service, and Admin roles. Write operations SHALL be allowed for the Admin role only.

The service SHALL return `401` for an unauthenticated request and `403` for a role without access.

#### Scenario: Service reads data

- GIVEN a caller with the Service role
- WHEN the caller sends `GET /repos/acme/widgets/tasks/fix-bug/tests/basic/data`
- THEN the status is `200`

#### Scenario: Service writes data

- GIVEN a caller with the Service role
- WHEN the caller sends `POST /repos/acme/widgets/tasks/fix-bug/tests/basic/data`
- THEN the status is `403`
- AND no datum is created
