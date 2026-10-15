# Schema Design

## Repo

Tasks are grouped by repo.

* Stub
* Org stub
* URL

## Tasks

Organizes tests around a use case.

* Stub
* Name
* Description
* Repo - the URL of the repo that holds the task
* Parent - Repo

## Tests

A collection of data that can be executed and graded as a group.

* Stub
* Name
* Description
* Version
* Schema - A list of keys found in scores
  * Stub - The key stored in data
  * Name - What the UI displays
* Parent - Task

## Data

JSON data passed to agents. Represents a single task the agent should perform.

* Ref - A short, Git-ref-like identifier
* JSON
* Parent - Test

### Files

Tasks that process files should include URLs and checksums in data. Agents and graders are responsible for downloading and verifying files.

## Model

Passed by API calls to agent and grader runs.

* Stub
* Name
* Version
* Variant
* Provider

## Agent

External code that can produce a task result. Runs in an Argo Workflow from a cloned Git repo. Agents are expected to be commands that read model config and data from stdin and write a boundary marker followed by a JSON result to stdout.

* Commit
* Version
* Build command
* Run command
* Parent - Task

## Result

The JSON produced by an agent.

* Ref
* JSON
* Logs
* Input tokens
* Output tokens
* Token cost
* Run duration
* Local - optional local branch that created the result
* Parent - Data

## Grader

External code that can score a result. Runs in an Argo Workflow from a cloned Git repo. Graders are expected to be commands that read model config, data, and a result from stdin and write a boundary marker followed by a JSON score to stdout.

* Commit
* Version
* Build command
* Run command
* Parent - Task

## Score

A key-number dictionary reporting how well an agent performed a task.

* Ref
* Key-number dictionary
* Logs
* Input tokens
* Output tokens
* Token cost
* Run duration
* Parent - Result
