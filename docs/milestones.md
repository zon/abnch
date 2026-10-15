# Milestones

## Graders and Scores

* A grader scores a result
* A score is a key-number dictionary that matches the test schema
* `abnch score` scores a result, and `abnch` reads scores

## Deployment

* The Helm chart installs the service into a cluster
* Agent and grader runs execute as Argo workflows
* Kubernetes service accounts and GitHub workflows authenticate runs
* A deployment supplies model provider and file access
