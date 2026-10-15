# Score Service API Specification

## Purpose

Store and read the scores graders produce for results. A score belongs to a result. Error shapes and authorization follow the [Repo Service API](repo.md). Grader config and grade runs are out of scope.

## Requirements

### Requirement: Score Identity

A score SHALL be identified by its result and a ref. The ref SHALL be a short, URL-safe string derived from the score's key-number dictionary within its result.

Two scores with the same dictionary for the same result SHALL have the same ref and SHALL be the same score. Scores for different results SHALL have different identities even when their dictionaries match.

#### Scenario: Same dictionary, same ref

- GIVEN a score with dictionary `{"correctness": 1}` exists for a result
- WHEN an authorized client adds the same dictionary for the same result
- THEN the service reports the same ref
- AND the result still holds one score

#### Scenario: Same dictionary, different result

- GIVEN a score exists for one result in a repo
- WHEN an authorized client adds the same dictionary for another result
- THEN the service reports a different ref

### Requirement: Score Fields

A score SHALL carry its key-number dictionary, logs, input tokens, output tokens, token cost, and run duration.

Token counts, token cost, and run duration SHALL be numbers that are zero or greater. A score that omits them SHALL record zero.

#### Scenario: Score carries its fields

- GIVEN a score exists with a dictionary, logs, token counts, a token cost, and a run duration
- WHEN an authorized client gets it
- THEN the body includes the dictionary, the logs, the tokens, the cost, and the duration

#### Scenario: Fields default to zero

- GIVEN an authorized client adds a score without token counts, a cost, or a duration
- WHEN the service stores it
- THEN the token counts, the cost, and the duration are zero

#### Scenario: Negative count

- GIVEN an authorized client adds a score with a negative token count
- WHEN the service handles it
- THEN the status is `422`
- AND no score is created

### Requirement: Score Schema

A score SHALL match the score schema of the test that owns its result, as defined in the [Test Service API](test.md). Every key in the dictionary SHALL be a key in the schema, and every value SHALL be a number. A score MAY omit a schema key.

A dictionary with a key outside the schema, or a value that is not a number, SHALL be an error and the service SHALL NOT create the score.

#### Scenario: Known keys

- GIVEN the result's test has schema keys `correctness` and `style`
- WHEN an authorized client adds a score with `{"correctness": 1, "style": 0.5}`
- THEN the status is `2xx`
- AND the score is created

#### Scenario: Partial score

- GIVEN the result's test has schema keys `correctness` and `style`
- WHEN an authorized client adds a score with `{"correctness": 1}`
- THEN the score is created

#### Scenario: Unknown key

- GIVEN the result's test has no schema key `speed`
- WHEN an authorized client adds a score with `{"speed": 1}`
- THEN the status is `422`
- AND no score is created

#### Scenario: Non-number value

- GIVEN the result's test has schema key `correctness`
- WHEN an authorized client adds a score with `{"correctness": "high"}`
- THEN the status is `422`
- AND no score is created

### Requirement: Score Parent

A score SHALL belong to a result. Adding a score to a missing result SHALL be an error, and the service SHALL NOT create the result or the score.

#### Scenario: Add a score to a missing result

- GIVEN no result `<ref>` exists in `acme/widgets`
- WHEN an authorized client adds a score for it
- THEN the status is `404`
- AND no result or score is created

### Requirement: Add Score

`POST /repos/{org}/{repo}/scores` SHALL add a score from the request body and return its ref.

The body SHALL carry the task stub, the test stub, the result ref, and the score dictionary. It MAY carry logs, input tokens, output tokens, token cost, and run duration.

The service SHALL derive the ref from the result and the dictionary, so adding the same dictionary for the same result again returns the existing ref and creates no second score.

The service SHALL return `201` for a new ref and `200` when the ref already exists. A malformed body SHALL return `422`. A missing result SHALL return `404`.

#### Scenario: Add a score

- GIVEN a result `<ref>` exists in `acme/widgets`
- WHEN an authorized client sends `POST /repos/acme/widgets/scores` with the task, the test, the result ref, and a dictionary
- THEN the status is `201`
- AND the body carries the score's ref
- AND a later `GET` returns the score

#### Scenario: Add the same score twice

- GIVEN a score with dictionary `{"correctness": 1}` exists for a result
- WHEN an authorized client posts the same dictionary for the same result
- THEN the status is `200`
- AND the service reports the existing ref
- AND the result holds one score

#### Scenario: Malformed body

- GIVEN a result exists
- WHEN an authorized client sends `POST /repos/acme/widgets/scores` with a body that carries no dictionary
- THEN the status is `422`
- AND no score is created

#### Scenario: Missing result

- GIVEN no result `<ref>` exists
- WHEN an authorized client sends `POST /repos/acme/widgets/scores` for it
- THEN the status is `404`
- AND no score is created

### Requirement: List Scores

`GET /repos/{org}/{repo}/scores` SHALL return the repo's score refs as a JSON array.

#### Scenario: List scores

- GIVEN two scores exist in `acme/widgets`
- WHEN an authorized client sends `GET /repos/acme/widgets/scores`
- THEN the status is `200`
- AND the body lists both refs

#### Scenario: List scores in a missing repo

- GIVEN no repo `acme/widgets` exists
- WHEN an authorized client sends `GET /repos/acme/widgets/scores`
- THEN the status is `404`

### Requirement: Get Score

`GET /repos/{org}/{repo}/scores/{ref}` SHALL return one score as JSON with its ref, task, test, result ref, dictionary, logs, tokens, cost, and duration.

#### Scenario: Get an existing score

- GIVEN a score `<ref>` exists in `acme/widgets`
- WHEN an authorized client sends `GET /repos/acme/widgets/scores/<ref>`
- THEN the status is `200`
- AND the body includes the ref, the task, the test, the result ref, the dictionary, the logs, the tokens, the cost, and the duration

#### Scenario: Get a missing score

- GIVEN no score with that ref exists
- WHEN an authorized client sends `GET /repos/acme/widgets/scores/<ref>`
- THEN the status is `404`

### Requirement: Authorization

Read operations SHALL be allowed for the Reader, Service, and Admin roles. Write operations SHALL be allowed for the Service and Admin roles.

The service SHALL return `401` for an unauthenticated request and `403` for a role without access.

#### Scenario: Reader reads a score

- GIVEN a caller with the Reader role
- WHEN the caller sends `GET /repos/acme/widgets/scores`
- THEN the status is `200`

#### Scenario: Service adds a score

- GIVEN a caller with the Service role
- WHEN the caller sends `POST /repos/acme/widgets/scores` with a valid body
- THEN the status is `2xx`

#### Scenario: Reader adds a score

- GIVEN a caller with the Reader role
- WHEN the caller sends `POST /repos/acme/widgets/scores` with a valid body
- THEN the status is `403`
- AND no score is created
