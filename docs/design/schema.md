# Schema Design

All records contain primary key ids. User experience should use stubs and refs instead of ids when possible.

## Repo

Tasks are grouped by repo. Unique by stub and and org stub.

* Stub
* Org stub
* URL

## Tasks

Organizes tests around a use case. Unique by stub and repo.

* Stub
* Name
* Description
* Repo - Related repo

## Tests

A collection of data that can be executed and graded as a group. Unique by stub and task.

* Stub
* Name
* Description
* Version
* Schema - A list of keys found in scores
  * Stub - The key stored in scores
  * Name - What the UI displays
* Task - Related task

## Data

JSON data passed to agents. Represents a single task the agent should perform. Unique by ref and test.

* Ref - A short, Git-ref-like identifier derived from the JSON. The same JSON always has the same ref
* JSON
* Test - Related test this data is for

### Files

Tasks that process files should include file references in data. A file reference is a JSON object carrying a `url` and a `checksum`. Agents and graders download and verify the files.

## Model

Used by results and scores. A result and a score link to the model record they used. Unique by name, provider, and variant.

* Name
* Provider
* Variant

## Agent

External code that can produce a task result. An agent record identifies the code by its commit and version. Unique by stub, commit, branch, version, uncommitted, and task.

* Name
* Stub
* Commit - Git commit
* Branch - Git branch name
* Version - optional version
* Uncommitted - if this agent ran from uncommitted code
* Task - relationship to the task this agent performs

## Result

The body an agent produced.

* Body
* Logs
* Input tokens
* Output tokens
* Token cost
* Run duration
* Model - relationship to the model record the agent used
* Agent - relationship to the agent record that produced the result
* Data - relationship to data record passed to agent

## Grader

External code that can score a result. A grader record identifies the code by its commit and version. Unique by stub, commit, branch, version, uncommitted, and test.

* Name
* Stub
* Commit - Git commit
* Branch - Git branch name
* Version - optional version
* Uncommitted - if this grader ran from uncommitted code
* Test - relationship to the test this grader scores

## Score

The body a grader produced for a result. It holds a number per key of the test score schema.

* Body
* Logs
* Input tokens
* Output tokens
* Token cost
* Run duration
* Model - relationship to the model record the grader used
* Grader - relationship to the grader record that produced the score
* Result - relationship to the result record being scored
