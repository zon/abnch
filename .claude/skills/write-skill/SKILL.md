---
name: write-skill
description: Writes an agent skill into the central skills/ directory following the Agent Skills spec. Use when the user wants to add a new skill or edit an existing one.
---

# Write Skill

Write a skill that a coding agent can invoke by name in any repository that installs it.

## Steps

1. **Read the [Agent Skills spec](https://agentskills.io/specification)** defining the skill format.

2. **Ask for the task and output** if the user's request doesn't state them.

3. **Read [docs/zpecs/prompts.md](docs/zpecs/prompts.md).** Apply its structure: one task, numbered steps, explicit output.

4. **Write the skill** to `skills/<name>/SKILL.md`.

   - **Name it for the task.** Join lowercase words with single hyphens, at most 64 characters, with no leading, trailing, or doubled hyphen. The name matches the directory and avoids the reserved words `anthropic` and `claude`.
   - **Write the description for discovery.** State what the skill does and when to use it, in third person, at most 1024 characters. Lead with the words a user would say, because a runner selects a skill on this field alone.
   - **Reference documentation rather than repeating it.** A skill that restates a format goes stale the moment the format changes.
   - **Link documents at their installed path.** Documents in this repository live at `docs/zpecs/<file>.md` and install to the same path in the target repository, so a relative markdown link resolves in both places. Use markdown links, never bare paths or code spans.
   - **Leave target-repository paths unlinked.** Paths like `./specs/<path>.md` or `specs/architecture.yaml` resolve wherever the skill runs.
   - **Use the shared frontmatter fields.** The spec requires `name` and `description`. Both Claude and opencode read the optional fields `license`, `compatibility`, and `metadata`. `allowed-tools` is experimental, so leave it out.
   - **Keep the body short.** A runner loads the whole file once it triggers the skill, so stay under 500 lines. Move longer material to a reference file beside it and link it once from `SKILL.md`.

5. **Report** the skill path, the situations that trigger it, and a one-line summary.
