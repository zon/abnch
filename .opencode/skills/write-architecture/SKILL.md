---
name: write-architecture
description: Creates or edits the architecture document at specs/architecture.yaml, recording the components that already exist in the codebase. Use when the user wants to document the current architecture.
---

# Write Architecture

Write the architecture document at **`specs/architecture.yaml`**, recording the components that already exist, not as a plan for code to come.

## Steps

1. **Read the architecture format docs** at [docs/zpecs/architecture-outline.md](docs/zpecs/architecture-outline.md).

2. **Read the existing architecture document** at the target path if one exists, so edits preserve unrelated components.

3. **Clarify the scope.** If the user's request is vague, ask clarifying questions before proceeding.

4. **Survey the codebase.** Find the components the application is built from, as the [Architecture Format](docs/zpecs/architecture-outline.md) defines. For each candidate component, confirm its path, responsibilities.

5. **Draft the architecture** following the format in step 1.

6. **Write the file** to the target path.

7. **Report** the file path and a one-line summary of the components covered.
