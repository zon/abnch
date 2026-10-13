# Task Service API Specification

## Purpose

Manage tasks over the service API. A task belongs to a repo and describes a use case. Error shapes and authorization follow the [Repo Service API](repo.md).

## Requirements

### Requirement: Task Identity

A task SHALL be identified by its repo and a task stub. The task stub SHALL be non-empty and URL-safe.

Two tasks in the same repo with the same stub SHALL be the same task.

#### Scenario: Same identity across writes

- GIVEN a task `acme/widgets/fix-bug` exists
- WHEN a client updates `acme/widgets/fix-bug`
- THEN the update applies to the same task
- AND no second task is created

### Requirement: List Tasks

`GET /repos/{org}/{repo}/tasks` SHALL return the repo's tasks as a JSON array.

#### Scenario: List tasks in a repo

- GIVEN two tasks exist in `acme/widgets`
- WHEN an authorized client sends `GET /repos/acme/widgets/tasks`
- THEN the status is `200`
- AND the body lists both tasks

#### Scenario: List tasks in a missing repo

- GIVEN no repo `acme/widgets` exists
- WHEN an authorized client sends `GET /repos/acme/widgets/tasks`
- THEN the status is `404`

### Requirement: Get Task

`GET /repos/{org}/{repo}/tasks/{stub}` SHALL return one task as JSON with its stub, name, description, and repo.

#### Scenario: Get an existing task

- GIVEN a task `acme/widgets/fix-bug` exists
- WHEN an authorized client sends `GET /repos/acme/widgets/tasks/fix-bug`
- THEN the status is `200`
- AND the body includes the stub, name, description, and repo

#### Scenario: Get a missing task

- GIVEN no task `acme/widgets/fix-bug` exists
- WHEN an authorized client sends `GET /repos/acme/widgets/tasks/fix-bug`
- THEN the status is `404`

### Requirement: Task Requires an Existing Repo

A repo SHALL exist before tasks can be written to it. Writing a task to a missing repo SHALL be an error, and the service SHALL NOT create the repo.

#### Scenario: Write a task to a missing repo

- GIVEN no repo `acme/widgets` exists
- WHEN an authorized client sends `PUT /repos/acme/widgets/tasks/fix-bug`
- THEN the status is `404`
- AND no repo or task is created

### Requirement: Create or Update Task

`PUT /repos/{org}/{repo}/tasks/{stub}` SHALL create the task when it is missing and update it when it exists.

The body SHALL carry the task name and description. On update, the service SHALL merge the body into the existing task.

The service SHALL return `201` on create and `200` on update.

#### Scenario: Create a task

- GIVEN a repo `acme/widgets` exists and no task `fix-bug` exists
- WHEN an authorized client sends `PUT /repos/acme/widgets/tasks/fix-bug` with a name and description
- THEN the status is `201`
- AND a later `GET` returns the task

#### Scenario: Update a task

- GIVEN a task `acme/widgets/fix-bug` exists
- WHEN an authorized client sends `PUT /repos/acme/widgets/tasks/fix-bug` with a new description
- THEN the status is `200`
- AND the description is replaced
- AND the name is unchanged

#### Scenario: Missing required field on create

- GIVEN a repo `acme/widgets` exists and no task `fix-bug` exists
- WHEN an authorized client sends `PUT /repos/acme/widgets/tasks/fix-bug` without a name
- THEN the status is `422`
- AND no task is created

### Requirement: Rename Task

`POST /repos/{org}/{repo}/tasks/{stub}/move` SHALL rename a task. The body SHALL carry the new task stub.

The service SHALL return `200` on success, `404` when the task is missing, and `409` when a task with the new stub already exists.

#### Scenario: Rename a task

- GIVEN a task `acme/widgets/fix-bug` exists and no task `fix-typo` exists
- WHEN an authorized client sends `POST /repos/acme/widgets/tasks/fix-bug/move` with `fix-typo`
- THEN the status is `200`
- AND `GET /repos/acme/widgets/tasks/fix-typo` returns the task
- AND `GET /repos/acme/widgets/tasks/fix-bug` returns `404`

#### Scenario: Rename onto an existing stub

- GIVEN tasks `fix-bug` and `fix-typo` exist in `acme/widgets`
- WHEN an authorized client moves `fix-bug` to `fix-typo`
- THEN the status is `409`
- AND both tasks are unchanged

### Requirement: Authorization

Read operations SHALL be allowed for the Reader, Service, and Admin roles. Write operations SHALL be allowed for the Admin role only.

The service SHALL return `401` for an unauthenticated request and `403` for a role without access.

#### Scenario: Service reads a task

- GIVEN a caller with the Service role
- WHEN the caller sends `GET /repos/acme/widgets/tasks/fix-bug`
- THEN the status is `200`

#### Scenario: Service writes a task

- GIVEN a caller with the Service role
- WHEN the caller sends `PUT /repos/acme/widgets/tasks/fix-bug`
- THEN the status is `403`
- AND no task is created
