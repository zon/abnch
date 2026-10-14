# Milestones

## Core Service

* `abnchd` runs from a local service config and connects to Postgres
* The service authenticates human users and workloads, then grants roles
* The service manages repos and tasks over its API
* `abnch` signs in and manages tasks in the repo it infers from the working directory

## Tests and Data

* A task owns tests, each with a version and a score schema
* A test owns JSON data
* Data can reference files by URL and checksum
* `abnch` manages tests and data

## Models and Agents

* The config records models
* An agent runs against a task's data and produces a result
* A result records the JSON, logs, tokens, cost, and run duration
* `abnch run` runs a test, and `abnch` reads results

## Graders and Scores

* A grader scores a result
* A score is a key-number dictionary that matches the test schema
* `abnch score` scores a result, and `abnch` reads scores

## Deployment

* The Helm chart installs the service into a cluster
* Agent and grader runs execute as Argo workflows
* Kubernetes service accounts and GitHub workflows authenticate runs
* A deployment supplies model provider and file access
