# Code Guidelines

We want clear, minimal code that is easy to understand and replace.

## Writing

* Each function and block should only do one specific thing
* Match current requirements and standards instead of the past or future
* Avoid functions with deep nesting. Return early or consider better architecture to keep code shallow and easy to read
* Code should be self-evident. Only write comments to summarize when clear code is infeasible

## Refactoring

* Gradually move towards ideal architecture and implementation. Always look for clearer, more minimal solutions
* Refactor immediately when the opportunity is inside components we're already working on
* Write a project when the refactoring touches components outside the current work. Name it `refactor-<slug>.yaml`

### Refactoring projects

Before writing a project, check whether a similar one already exists in `./projects/`. If it does, update it instead of writing a new one. Use the `write-project` skill.