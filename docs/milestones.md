# Milestones

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
