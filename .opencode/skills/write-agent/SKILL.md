---
name: write-agent
description: Writes an agent definition into the central agents/ directory following the Agent Format. Use when the user wants to add a new agent or edit an existing one.
---

# Write Agent

Write an agent definition that a runner renders into a target project's `.claude` or `.opencode` directory.

## Steps

1. **Read the [Agent Format](docs/cli/agent-format.md)** defining the frontmatter fields and the body.

2. **Ask for the task and output** if the user's request doesn't state them.

3. **Read [docs/zpecs/prompts.md](docs/zpecs/prompts.md).** The body becomes the system prompt, so use the same structure: role, task, context, numbered instructions, output.

4. **Write the definition** to `agents/<name>.md`.

   - **Keep the name short and matching the file name.**
   - **Write the description as a trigger**, naming the situations that call the agent. A runner matches an agent on its description, like a skill.
   - **List only the tools the agent needs**, or omit the list when the agent needs every tool.
   - **Set `mode` to `primary`** when the agent works on its own, and leave it off when another agent calls it.
   - **Write the body as the system prompt.** Assign the agent its role and give numbered instructions. Reference documents by path rather than restating their contents, so the definition does not go stale when a format changes.

5. **Report** the file path, the situations that trigger it, and a one-line summary.
