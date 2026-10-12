# Glossary

Terms used throughout these documents.

## App

A runnable the project ships, like a server, worker, or CLI. Apps can group [specs](specs.md) by the runtime that serves them.

## Component

A collection of resources in the repo that build the project. See [Architecture Format](architecture-outline.md).

## Feature

A coherent slice of user-facing or system-facing behavior: something a user can do, or something the system does for them.

## Requirement

A single behavior the system must have. Requirements live in a [spec](specs.md) or a [project](project.md).

## Scenario

A concrete example of a requirement in action, in Given/When/Then form. Scenarios live in a [spec](specs.md).

## Project

A YAML or JSON file listing requirements for a coding agent. See [Project Format](project.md).

## Deep Module

A module with a simple interface over a complex implementation.

## Pure Module

A module that contains only value objects and pure functions: no I/O, no shared state, no external calls.

## Implementation Module

A module that does the low-level work: database queries, API calls, file I/O, data transformations. See [Architecture Guidelines](architecture.md).
