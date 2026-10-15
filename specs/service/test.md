# Test Service API Specification

## Purpose

Manage tests over the service API. A test belongs to a task and groups the data and score schema for one use case. Error shapes and authorization follow the [Repo Service API](repo.md).

## Requirements

### Requirement: Test Identity

A test SHALL be identified by its repo, task stub, and test stub. The test stub SHALL be non-empty and URL-safe.

Two tests in the same task with the same stub SHALL be the same test.

#### Scenario: Same identity across writes

- GIVEN a test `acme/widgets/fix-bug/basic` exists
- WHEN a client updates `acme/widgets/fix-bug/basic`
- THEN the update applies to the same test
- AND no second test is created

### Requirement: Test Fields

A test SHALL have a name, a version, and a score schema.

The name SHALL be non-empty. The version SHALL be a non-empty string. An empty schema SHALL be allowed.

#### Scenario: Test carries its fields

- GIVEN a test `acme/widgets/fix-bug/basic` exists with a name, a version, and a schema
- WHEN an authorized client sends `GET /repos/acme/widgets/tasks/fix-bug/tests/basic`
- THEN the body includes the name, the version, and the schema

### Requirement: Score Schema

A score schema SHALL be an ordered list of keys. Each key SHALL have a stub, the key stored in scores, and a name for display. Stubs SHALL be unique within a schema.

#### Scenario: Schema keys

- GIVEN a test with schema keys `correctness` and `style`
- WHEN an authorized client gets the test
- THEN the schema lists both keys with their stubs and names

#### Scenario: Duplicate key stub

- GIVEN a task exists
- WHEN an authorized client sends `PUT /repos/acme/widgets/tasks/fix-bug/tests/basic` with two schema keys sharing a stub
- THEN the status is `422`
- AND no test is created

### Requirement: List Tests

`GET /repos/{org}/{repo}/tasks/{task}/tests` SHALL return the task's tests as a JSON array.

#### Scenario: List tests in a task

- GIVEN two tests exist in `acme/widgets/fix-bug`
- WHEN an authorized client sends `GET /repos/acme/widgets/tasks/fix-bug/tests`
- THEN the status is `200`
- AND the body lists both tests

#### Scenario: List tests in a missing task

- GIVEN no task `acme/widgets/fix-bug` exists
- WHEN an authorized client sends `GET /repos/acme/widgets/tasks/fix-bug/tests`
- THEN the status is `404`

### Requirement: Get Test

`GET /repos/{org}/{repo}/tasks/{task}/tests/{stub}` SHALL return one test as JSON with its stub, name, description, version, and schema.

#### Scenario: Get an existing test

- GIVEN a test `acme/widgets/fix-bug/basic` exists
- WHEN an authorized client sends `GET /repos/acme/widgets/tasks/fix-bug/tests/basic`
- THEN the status is `200`
- AND the body includes the stub, name, description, version, and schema

#### Scenario: Get a missing test

- GIVEN no test `acme/widgets/fix-bug/basic` exists
- WHEN an authorized client sends `GET /repos/acme/widgets/tasks/fix-bug/tests/basic`
- THEN the status is `404`

### Requirement: Test Requires an Existing Task

A task SHALL exist before tests can be written to it. Writing a test to a missing task SHALL be an error, and the service SHALL NOT create the task.

#### Scenario: Write a test to a missing task

- GIVEN no task `acme/widgets/fix-bug` exists
- WHEN an authorized client sends `PUT /repos/acme/widgets/tasks/fix-bug/tests/basic`
- THEN the status is `404`
- AND no task or test is created

### Requirement: Create or Update Test

`PUT /repos/{org}/{repo}/tasks/{task}/tests/{stub}` SHALL create the test when it is missing and update it when it exists.

The body SHALL carry the test name, description, version, and schema. On update, the service SHALL merge the body into the existing test. A body that carries a schema SHALL replace the existing schema.

The service SHALL return `201` on create and `200` on update.

#### Scenario: Create a test

- GIVEN a task `acme/widgets/fix-bug` exists and no test `basic` exists
- WHEN an authorized client sends `PUT /repos/acme/widgets/tasks/fix-bug/tests/basic` with a name, a version, and a schema
- THEN the status is `201`
- AND a later `GET` returns the test

#### Scenario: Update a test

- GIVEN a test `acme/widgets/fix-bug/basic` exists
- WHEN an authorized client sends `PUT /repos/acme/widgets/tasks/fix-bug/tests/basic` with a new version
- THEN the status is `200`
- AND the version is replaced
- AND the name is unchanged

#### Scenario: Replace the schema

- GIVEN a test `acme/widgets/fix-bug/basic` exists with a schema
- WHEN an authorized client sends `PUT /repos/acme/widgets/tasks/fix-bug/tests/basic` with a new schema
- THEN the schema is replaced

#### Scenario: Missing required field on create

- GIVEN a task `acme/widgets/fix-bug` exists and no test `basic` exists
- WHEN an authorized client sends `PUT /repos/acme/widgets/tasks/fix-bug/tests/basic` without a name or a version
- THEN the status is `422`
- AND no test is created

### Requirement: Rename Test

`POST /repos/{org}/{repo}/tasks/{task}/tests/{stub}/move` SHALL rename a test. The body SHALL carry the new test stub.

The service SHALL return `200` on success, `404` when the test is missing, and `409` when a test with the new stub already exists.

#### Scenario: Rename a test

- GIVEN a test `acme/widgets/fix-bug/basic` exists and no test `smoke` exists
- WHEN an authorized client sends `POST /repos/acme/widgets/tasks/fix-bug/tests/basic/move` with `smoke`
- THEN the status is `200`
- AND `GET /repos/acme/widgets/tasks/fix-bug/tests/smoke` returns the test
- AND `GET /repos/acme/widgets/tasks/fix-bug/tests/basic` returns `404`

#### Scenario: Rename onto an existing stub

- GIVEN tests `basic` and `smoke` exist in `acme/widgets/fix-bug`
- WHEN an authorized client moves `basic` to `smoke`
- THEN the status is `409`
- AND both tests are unchanged

### Requirement: Authorization

Read operations SHALL be allowed for the Reader, Service, and Admin roles. Write operations SHALL be allowed for the Admin role only.

The service SHALL return `401` for an unauthenticated request and `403` for a role without access.

#### Scenario: Service reads a test

- GIVEN a caller with the Service role
- WHEN the caller sends `GET /repos/acme/widgets/tasks/fix-bug/tests/basic`
- THEN the status is `200`

#### Scenario: Service writes a test

- GIVEN a caller with the Service role
- WHEN the caller sends `PUT /repos/acme/widgets/tasks/fix-bug/tests/basic`
- THEN the status is `403`
- AND no test is created
