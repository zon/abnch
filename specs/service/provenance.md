# Provenance Service API Specification

## Purpose

Store the model, agent, and grader records that results and scores reference. A result links to the model and agent that produced it. A score links to the model and grader that produced it. The service creates or reuses the records from the records a result or score carries. Results are defined in the [Result Service API](result.md) and scores in the [Score Service API](score.md).

## Requirements

### Requirement: Model Record

A model record SHALL carry a name, a provider, and a variant.

Two model values with the same name, provider, and variant SHALL be the same record.

The service SHALL create or reuse the model record from the model a result or score carries. A result or score that carries a model without a name or provider SHALL be rejected with `422`, and the service SHALL create nothing.

#### Scenario: Same model, same record

- GIVEN a result or score carries model name `claude-sonnet`, provider `anthropic`, and variant `high`
- WHEN the service stores it
- THEN the result or score links to a model record
- AND a later result or score with the same model values links to the same record

#### Scenario: Different variant, different record

- GIVEN a model record exists for variant `high`
- WHEN a result or score carries the same name and provider with variant `low`
- THEN it links to a different model record

#### Scenario: Missing name or provider

- GIVEN a result or score carries a model without a name
- WHEN the service handles it
- THEN the status is `422`
- AND nothing is created

### Requirement: Agent Record

An agent record SHALL carry a name, a stub, a commit, a branch, an optional version, an uncommitted flag that defaults to false, and a task. The service SHALL associate the agent with the task that owns the result's datum.

Two agent values with the same stub, commit, branch, version, uncommitted flag, and task SHALL be the same record.

The service SHALL create or reuse the agent record from the agent a result carries. A result that carries an agent without a name, stub, commit, or branch SHALL be rejected with `422`, and the service SHALL create nothing.

#### Scenario: Same agent, same record

- GIVEN a result carries an agent with name `claude-code`, stub `claude`, commit `abc123`, branch `main`, version `1.2.3`, and uncommitted false
- WHEN the service stores it
- THEN the result links to an agent record
- AND a later result with the same agent values for the same task links to the same record

#### Scenario: Different commit, different record

- GIVEN an agent record exists for commit `abc123`
- WHEN a result carries the same agent with commit `def456`
- THEN the result links to a different agent record

#### Scenario: Uncommitted code, different record

- GIVEN an agent record exists with uncommitted false
- WHEN a result carries the same agent with uncommitted true
- THEN the result links to a different agent record

#### Scenario: Missing name, stub, commit, or branch

- GIVEN a result carries an agent without a commit
- WHEN the service handles it
- THEN the status is `422`
- AND no result or agent record is created

### Requirement: Grader Record

A grader record SHALL carry a name, a stub, a commit, a branch, an optional version, an uncommitted flag that defaults to false, and a test. The service SHALL associate the grader with the test that owns the result being scored.

Two grader values with the same stub, commit, branch, version, uncommitted flag, and test SHALL be the same record.

The service SHALL create or reuse the grader record from the grader a score carries. A score that carries a grader without a name, stub, commit, or branch SHALL be rejected with `422`, and the service SHALL create nothing.

#### Scenario: Same grader, same record

- GIVEN a score carries a grader with name `llm-judge`, stub `judge`, commit `def456`, branch `main`, version `2.0.0`, and uncommitted false
- WHEN the service stores it
- THEN the score links to a grader record
- AND a later score with the same grader values for the same test links to the same record

#### Scenario: Different commit, different record

- GIVEN a grader record exists for commit `def456`
- WHEN a score carries the same grader with commit `abc123`
- THEN the score links to a different grader record

#### Scenario: Uncommitted code, different record

- GIVEN a grader record exists with uncommitted false
- WHEN a score carries the same grader with uncommitted true
- THEN the score links to a different grader record

#### Scenario: Missing name, stub, commit, or branch

- GIVEN a score carries a grader without a commit
- WHEN the service handles it
- THEN the status is `422`
- AND no score or grader record is created

### Requirement: Record Links

A result SHALL link to one model record and one agent record. A score SHALL link to one model record and one grader record.

The result body SHALL carry the model and agent records. The score body SHALL carry the model and grader records. The service SHALL create or reuse the records as part of the write.

A response carrying one result or one score SHALL include the linked records.

#### Scenario: Result links to its records

- GIVEN a result carries a model and an agent
- WHEN an authorized client gets the result
- THEN the body includes the model record and the agent record

#### Scenario: Score links to its records

- GIVEN a score carries a model and a grader
- WHEN an authorized client gets the score
- THEN the body includes the model record and the grader record

### Requirement: Authorization

Result and score writes SHALL be allowed for the Service and Admin roles. Result and score reads SHALL be allowed for the Reader, Service, and Admin roles.

The service SHALL return `401` for an unauthenticated request and `403` for a role without access.

#### Scenario: Reader reads a result

- GIVEN a caller with the Reader role
- WHEN the caller gets a result
- THEN the status is `200`
- AND the body includes the model and agent records

#### Scenario: Service adds a result

- GIVEN a caller with the Service role
- WHEN the caller posts a result with a model and an agent
- THEN the status is `2xx`
- AND the model and agent records are created or reused

#### Scenario: Reader adds a result

- GIVEN a caller with the Reader role
- WHEN the caller posts a result
- THEN the status is `403`
- AND no result or record is created
