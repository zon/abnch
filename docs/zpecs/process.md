# Development Process

We use a layered, spec-based development process:

* Milestones
* Design Docs
* Specs
* Projects
* Implementation
* Regressions

The objective of this process is to put a repo into equilibrium where no work remains.

## Milestones

High level objectives, recorded in `milestones.md`. Milestones are separated by section and we work on them in order. Milestone sections should be removed by the pull requests that complete them. An empty milestones file should just include a header and say, "No milestones."

## Design Docs

High level design docs in `design/` describing core concepts, their relationships, and key domain logic. Design docs define the language and structure of specs and code beneath them. A repo should gradually update its specs and code to follow the design.

## Specs

We use a [simple variant of OpenSpec](specs.md) to define domain logic requirements.

## Projects

We use [simple project files](project.md) to make implementation plans. Update design and specs before writing or updating a project, and keep those updates within the project's scope. The project covers only their implementation.

## Implementation

Projects are run with [Zon's Ralph](https://github.com/zon/ralph).

## Regressions

Record bugs or chores found outside a project's scope in `regressions.md`.

## Equilibrium

To pick what to do next, check the following and address one gap:

* Are there regressions? Pick one and address it
* Is a project missing for the current design and specs? Recommend a project
* Is design ahead? Pick a focused scope. Update specs. Write a project
* Are specs ahead? Pick a focused scope. Write a project
* Are there gaps in design? Pick a focused scope. Walk through filling the gap
* Do we need a milestone?
