---
name: write-spec
description: Creates a spec document in /specs. Use when the user wants to write a spec, plan a feature spec, document behavior requirements, or add scenarios for a feature area.
---

# Write Spec

Create a well-formed spec file in `./specs/` based on the user's description of the feature or behavior area.

## Steps

1. **Understand the scope.** If the user's request is vague, ask clarifying questions:
   - What feature or behavior area does this spec cover?
   - Which component does it belong to?
   - Is this a new feature or documenting existing behavior?
   - Are there known edge cases or failure modes to capture?

2. **Read the spec format docs** to refresh your understanding:
   - [docs/zpecs/specs.md](docs/zpecs/specs.md)

3. **Determine the file path.** Check the existing `specs/` structure to match its convention as described in [docs/zpecs/specs.md](docs/zpecs/specs.md#file-location).

4. **Choose the level** as described in [docs/zpecs/specs.md](docs/zpecs/specs.md) (default to Lite).

5. **Draft the spec** following the format and guidelines in [docs/zpecs/specs.md](docs/zpecs/specs.md).

6. **Write the file** to `./specs/<path>.md`, where the last path segment names the spec.

7. **Update `specs/README.md`.** Add or update a list item with the spec name as a relative link to the spec, followed by an em dash and a one-sentence description drawn from the spec's Purpose section. Mirror the spec's path in the section structure, creating sections if they don't exist yet.

8. **Report** the file path and a one-line summary of what the spec covers.
