---
name: write-project
description: Creates and validates a project file listing the requirements a coding agent works through, one per iteration. Use when the user wants to plan work for the agent or create or update a project file.
---

# Write Project

Create a well-formed project file based on the user's description of the work to be done.

## Steps

1. **Understand the work.** If the user's request is vague, ask clarifying questions:
   - What change does this project cover?
   - Does the work require a version bump?

2. **Read the format docs** to refresh your understanding:
   - [docs/zpecs/project.md](docs/zpecs/project.md)
   - [docs/zpecs/requirements.md](docs/zpecs/requirements.md)

3. **Draft the requirements.** Write one entry per iteration, stating the outcome rather than the implementation, and split anything that needs more than one round of work into separate entries. Cover the edge cases and operational requirements the user's description implies, and add the version bump if one is needed.

4. **Write the file** at `./projects/<slug>.yaml` as a top-level list of strings, following the format in [docs/zpecs/project.md](docs/zpecs/project.md).

5. **Validate.** Confirm the file parses as a list holding at least one non-empty entry. Empty entries, such as null, `false`, `0`, and blank strings, are typically dropped during resolution, so never leave a placeholder in the list. If the project's runner provides a validation command, use it.

6. **Report** the file path and a one-line summary of what the project covers.
