# Auth Design

## Authentication

* OAuth
  * Dev issuers
* OIDC
  * Human users

## Local Service Runtime

A service can run on a developer's machine. It trusts a dev issuer instead of a deployment's OIDC provider, so a local client can authenticate without a sign-in.

The CLI is a client on the same machine. `just config` generates its local credential. The CLI presents the credential to the local service. The dev issuer's role map grants the CLI Admin. See [Dev Config Design](dev-config.md).

## Authorization Roles

* Reader
  * Can get tasks
  * Can get tests and data
  * Can get results
  * Can get scores
* Admin
  * Full access

### Role Assignment

The service derives roles from claims on the authenticated credential. Each credential source has a role claim and a role map, configured in the [Service Config Design](app/abnchd-config.md). A claim value with no mapping grants no role.