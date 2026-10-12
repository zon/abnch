# Spec Format

The spec format describes system behavior with structured requirements and scenarios.

## File Location

Specs live at `/specs/<path>.md`. A spec path ends in a [feature](glossary.md#feature). The segments before it group the spec by [app](glossary.md#app), [component](glossary.md#component), or [feature](glossary.md#feature), in that order:

```
/specs/
└── <app>/<component>/<feature>.md
```

Groups are optional and repeatable, so a path may chain several components or several features:

```
/specs/auth.md                      # a feature
/specs/web/auth.md                  # app → feature
/specs/orders/checkout.md           # component → feature
/specs/web/orders/checkout.md       # app → component → feature
/specs/orders/billing/payments.md   # two components → feature
```

Within those rules, group by what fits the repo best.

Name features by what the system does (`auth`, `payments`, `notifications`), not how it does it (`jwt-handler`, `stripe-client`). If a feature grows too large to read comfortably, split it into sub-features rather than by implementation detail.

## Structure

A spec has requirements, each with scenarios:

```markdown
# Auth Specification

## Purpose
Authentication and session management for the application.

## Requirements

### Requirement: User Authentication
The system SHALL issue a JWT token upon successful login.

#### Scenario: Valid credentials
- GIVEN a user with valid credentials
- WHEN the user submits login form
- THEN a JWT token is returned
- AND the user is redirected to dashboard

#### Scenario: Invalid credentials
- GIVEN invalid credentials
- WHEN the user submits login form
- THEN an error message is displayed
- AND no token is issued

### Requirement: Session Expiration
The system MUST expire sessions after 30 minutes of inactivity.

#### Scenario: Idle timeout
- GIVEN an authenticated session
- WHEN 30 minutes pass without activity
- THEN the session is invalidated
- AND the user must re-authenticate
```

| Element | Purpose |
|---------|---------|
| `## Purpose` | High-level description of this spec's domain |
| `### Requirement:` | A behavior the system must have, stated without implementation |
| `#### Scenario:` | A concrete example of the requirement in action: testable, covering both the happy path and the edge cases |
| SHALL/MUST/SHOULD/MAY | RFC 2119 keywords: MUST and SHALL are absolute, SHOULD allows exceptions, MAY is optional |

## What a Spec Is (and Is Not)

A spec is a **behavior contract**, not an implementation plan.

Good spec content:
- Observable behavior users or downstream systems rely on
- Inputs, outputs, and error conditions
- External constraints (security, privacy, reliability, compatibility)
- Scenarios that can be tested or explicitly validated

Avoid in specs:
- Internal class/function names
- Library or framework choices
- Step-by-step implementation details
- Detailed execution plans. Those belong in a [project](project.md)

## Shared Contracts

When two specs rely on the same contract, define it in one place and link from the others. Link to the spec that owns the contract rather than restating it. Write the link as a markdown link, not a path in plain text.

## Keep It Light

Use the lightest level that still makes the change verifiable.

**Lite spec (default):**
- Short behavior-first requirements
- Clear scope and non-goals
- A few concrete acceptance checks

**Full spec (for higher risk):**
- Cross-team or cross-repo changes
- API/contract changes, migrations, security/privacy concerns
- Changes where ambiguity would be costly to fix later

Most changes should stay in Lite mode.
