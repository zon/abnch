# Result Design

A result is the body an agent produced for a datum. The CLI adds a result to the service.

## Models

A model is a record with a name, a provider, and a variant. A result carries the model it used. The service creates or reuses the model record and links the result to it.

## Agents

An agent is a record with a name, a stub, a commit, a branch, an optional version, and an uncommitted flag. A result carries the agent that produced it. The service associates the agent with the result's task, creates or reuses the record, and links the result to it.

## Result Input

`abnch add result <task>/<test>/<datum ref>` reads a result input as YAML or JSON from stdin or `--file`.

```yaml
model:
  name: claude-sonnet
  provider: anthropic
  variant: high
agent:
  name: claude-code
  stub: claude
  commit: abc123
  branch: main
  version: 1.2.3
  uncommitted: false
body: { answer: hi }
logs: resolving the prompt
inputTokens: 12
outputTokens: 3
tokenCost: 0.01
duration: 0.5
```

The `model`, `agent`, and `body` properties are required. The rest are optional.

## Provenance

A result links to the model and agent records that produced it. The model record carries the name, provider, and variant. The agent record carries the name, stub, commit, branch, version, and uncommitted flag. The service stores the records and the link, as defined in the [Schema Design](schema.md).
