# Auth Service API Specification

## Purpose

Authenticate callers to the service API and grant them roles. Authentication covers human users and non-human workloads. Authorization decides which operations a role may perform. Roles are defined in the [Auth Design](../../docs/design/auth.md). This spec owns the shared auth contract that the other service specs link to.

## Requirements

### Requirement: Authenticated Requests

Every request to the service API SHALL carry credentials. The service SHALL authenticate the request before handling it, and the request SHALL have exactly one authenticated caller and one set of roles.

A request with no credentials or with credentials the service cannot verify SHALL receive `401`, and the service SHALL perform no operation.

#### Scenario: Missing credentials

- GIVEN a request to any service route
- WHEN it carries no credentials
- THEN the status is `401`
- AND no operation is performed

#### Scenario: Unverifiable credentials

- GIVEN a request with a credential the service cannot verify
- WHEN the service handles it
- THEN the status is `401`
- AND no operation is performed

### Requirement: Workload Authentication

The service SHALL authenticate non-human callers with OAuth, without an interactive sign-in. A workload token SHALL identify the caller.

The service SHALL accept a workload token only from an issuer and audience configured in the [Service Config Design](../../docs/design/app/abnchd-config.md). A token with an unknown issuer, a wrong audience, a bad signature, or an expiry in the past SHALL be rejected.

#### Scenario: Kubernetes service account

- GIVEN a workload with a Kubernetes service account token
- WHEN it sends a request
- THEN the service authenticates the caller

#### Scenario: GitHub workflow

- GIVEN a GitHub workflow with a workflow token
- WHEN it sends a request
- THEN the service authenticates the caller

#### Scenario: Local dev issuer

- GIVEN a local client with a token signed by a dev issuer configured in a local service config
- WHEN it sends a request
- THEN the service authenticates the caller

#### Scenario: Unknown issuer

- GIVEN a workload token from an issuer the service does not trust
- WHEN it sends a request
- THEN the status is `401`
- AND no operation is performed

#### Scenario: Wrong audience

- GIVEN a workload token whose audience is not the service
- WHEN it sends a request
- THEN the status is `401`
- AND no operation is performed

#### Scenario: Rejected workload token

- GIVEN a workload token with a bad signature or an expiry in the past
- WHEN it sends a request
- THEN the status is `401`
- AND no operation is performed

### Requirement: Human Authentication

The service SHALL sign in human users through an OIDC provider using the authorization code flow, as configured in the [Service Config Design](../../docs/design/app/abnchd-config.md). On success the service SHALL grant a session that identifies the user and carries the user's roles. A failed sign-in SHALL grant no session.

A service whose config names no OIDC provider SHALL report that OIDC login is unavailable when a client starts a sign-in, and SHALL grant no session.

#### Scenario: Successful sign-in

- GIVEN a user with an account at the provider
- WHEN the user completes sign-in
- THEN the service grants a session for that user
- AND later requests with the session are authenticated

#### Scenario: Failed sign-in

- GIVEN a user who does not complete sign-in
- WHEN the provider returns an error or a rejection
- THEN the service grants no session

#### Scenario: Sign-in without a provider

- GIVEN a service whose config names no OIDC provider
- WHEN a client starts a sign-in
- THEN the service reports that OIDC login is unavailable
- AND the service grants no session

### Requirement: Role Assignment

The service SHALL derive roles from claims on the authenticated credential.

For OIDC users, the service SHALL read the role claim and map each value through the role map. For workloads, the service SHALL read the role claim and role map of the token's trusted issuer. A claim value with no mapping SHALL grant no role.

Humans hold Reader or Admin. Workloads hold Service, Agent, or Grader.

#### Scenario: Mapped user claim

- GIVEN a signed-in user whose role claim value maps to `Admin`
- WHEN the service assigns roles
- THEN the caller holds the Admin role

#### Scenario: Unmapped user claim

- GIVEN a signed-in user whose role claim value has no mapping
- WHEN the service assigns roles
- THEN the caller holds no role

#### Scenario: Mapped workload claim

- GIVEN a workload whose role claim value maps to `Service`
- WHEN the service assigns roles
- THEN the caller holds the Service role

#### Scenario: Unmapped workload claim

- GIVEN a workload whose role claim value has no mapping
- WHEN the service assigns roles
- THEN the caller holds no role

### Requirement: Reader Role

The Reader role SHALL allow reading tasks, tests, data, results, and scores. It SHALL NOT allow any write.

#### Scenario: Reader reads

- GIVEN a caller with the Reader role
- WHEN the caller reads a task, test, data, result, or score
- THEN the status is `2xx`

#### Scenario: Reader writes

- GIVEN a caller with the Reader role
- WHEN the caller writes any resource
- THEN the status is `403`

### Requirement: Service Role

The Service role SHALL allow reading tasks, tests, and data, posting results, and posting scores. Posting a result or score SHALL create and link the model, agent, and grader records it carries.

#### Scenario: Service posts a result

- GIVEN a caller with the Service role
- WHEN the caller posts a result
- THEN the status is `2xx`
- AND the result's model and agent records are created or reused

### Requirement: Agent and Grader Roles

The Agent and Grader roles SHALL be stateless. They SHALL allow a workload to reach the model providers configured for its run.

#### Scenario: Agent reaches a model provider

- GIVEN a workload running with the Agent role
- WHEN it calls a configured model provider
- THEN the call is allowed

#### Scenario: Grader reaches a model provider

- GIVEN a workload running with the Grader role
- WHEN it calls a configured model provider
- THEN the call is allowed

### Requirement: Admin Role

The Admin role SHALL allow every operation.

#### Scenario: Admin writes

- GIVEN a caller with the Admin role
- WHEN the caller performs any operation
- THEN the operation is allowed

### Requirement: Authorization Errors

An authenticated caller whose roles do not permit an operation SHALL receive `403`, and the service SHALL perform no part of the operation.

#### Scenario: Role without access

- GIVEN an authenticated caller whose roles do not permit an operation
- WHEN the caller attempts it
- THEN the status is `403`
- AND no part of the operation is performed
