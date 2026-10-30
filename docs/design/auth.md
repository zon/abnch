# Auth Design

## Authentication

* OAuth
  * K8s service accounts
  * GitHub workflows
  * Dev issuers
* OIDC
  * Human users

## Local Service Runtime

A service can run on a developer's machine. It trusts a dev issuer instead of a deployment's issuers, so a local client can authenticate without a Kubernetes or GitHub identity.

The CLI is a client on the same machine. `just config` generates its local credential. The CLI presents the credential to the local service. The dev issuer's role map grants the CLI Admin and a separate workload Service. See [Dev Config Design](dev-config.md).

## Authorization Roles

* Reader
  * Can get tasks
  * Can get tests and data
  * Can get results
  * Can get scores
* Admin
  * Full access
* Service
  * Can get tasks
  * Can get tests and data
  * Can post results
  * Can post scores, which create and link model, agent, and grader records
* Agent
  * Stateless
  * Can access model providers
* Grader
  * Stateless
  * Can access model providers

### Role Assignment

The service derives roles from claims on the authenticated credential. Each credential source has a role claim and a role map, configured in the [Service Config Design](app/abnchd-config.md). A claim value with no mapping grants no role.

Humans map to Reader or Admin. Workloads map to Service, Agent, or Grader.

For example:

* An agent tester service account maps to Service
* An agent workload maps to Agent
* A grader workload maps to Grader