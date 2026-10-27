---
name: update-dependency
description: Finds an outdated direct dependency, reads its release notes, and applies one update with verification. Use when the user asks to update, upgrade, or bump a dependency, or to check for outdated packages.
---

# Update Dependency

Apply one direct dependency update per run, so the change stays small enough to review.

## Steps

1. **List direct dependencies.** Read every dependency manifest in the repository and keep the dependencies the project declares for itself. Leave out transitive dependencies, including entries a manifest marks as indirect. A direct dependency comes from the project. A transitive one comes through another dependency, so the project cannot update it on its own.

   Version pins outside a manifest are direct dependencies too. Read container files, CI workflows, and install or infrastructure scripts. A base image tag, a tool version, or a version variable the project sets is a pin it owns.

2. **Order the dependencies.** Put small standalone dependencies first and large frameworks and platforms last.

3. **Find one update.** Use the package manager for each ecosystem to list outdated dependencies. Confirm against the package registry when the package manager cannot. For a pin outside a manifest, compare it with the registry, release page, or image tags for that project. Skip prereleases. Stop at the first direct dependency with a newer stable version. When no direct dependency has one, report that and stop, even if transitive dependencies do.

4. **Read the release notes.** Read the changelog for every skipped version and note breaking changes. A breaking change, a major version, or a changed component boundary, public API, or data model is significant. Otherwise the change is small.

5. **Apply the update.**
   - Update the manifest and lockfile with the package manager, not by hand.
   - Update a pin outside a manifest in the file that sets it, and in every place that repeats it.
   - For a small change, fix the code the update breaks.
   - For a significant change, write a spec and project as described in [docs/zpecs/specs.md](docs/zpecs/specs.md) and [docs/zpecs/project.md](docs/zpecs/project.md), and a design under `docs/design`, before coding.
   - Follow the repository's rules for changelogs and version bumps.

6. **Verify.** Run the build and tests, and fix what the update breaks before reporting. When the update changes a container image or a tool in it, build the image before reporting.

7. **Report** the dependency, the old and new versions, whether the change was small or significant, and the verification result.
