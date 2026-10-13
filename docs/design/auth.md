# Auth Design

## Authentication

* OAuth
  * K8s service accounts
  * GitHub workflows
* OIDC
  * Human users

## Authorization Roles

* Reader
  * Can get tasks
  * Can get results
  * Can get scores
* Admin
  * Full access
* Service
  * Can get tasks
  * Can run agent workflows
  * Can post results
  * Can run grader workflows
  * Can post scores
* Agent
  * Stateless
  * Can access model providers
* Grader
  * Stateless
  * Can access model providers

### Role Assignment

The service derives roles from claims on the authenticated credential. Each credential source has a role claim and a role map, configured in the [Service Config Design](service-config.md). A claim value with no mapping grants no role.

Humans map to Reader or Admin. Workloads map to Service, Agent, or Grader.

For example:

* An agent tester service account maps to Service
* An agent workflow maps to Agent
* A grader workflow maps to Grader