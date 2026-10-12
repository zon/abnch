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

* Humans
  * Reader
  * Admin
* Agent tester service
  * Service
* GitHub workflows
  * Admin
* Agent workflows
  * Agent
* Grader workflows
  * Grader