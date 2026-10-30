# Score Design

A score is a key-number dictionary reporting how well an agent performed a task. The CLI adds a score to the service.

## Models

A model is a record with a name, a provider, and a variant. A score carries the model it used. The service creates or reuses the model record and links the score to it.

## Graders

A grader is a record with a name, a stub, a commit, a branch, an optional version, and an uncommitted flag. A score carries the grader that produced it. The service associates the grader with the result's test, creates or reuses the record, and links the score to it.

## Score Input

`abnch add score <result ref>` reads a score input as YAML or JSON from stdin or `--file`.

```yaml
model:
  name: claude-sonnet
  provider: anthropic
  variant: high
grader:
  name: llm-judge
  stub: judge
  commit: def456
  branch: main
  version: 2.0.0
  uncommitted: false
dictionary: { correctness: 1 }
logs: grading the answer
inputTokens: 12
outputTokens: 3
tokenCost: 0.01
duration: 0.5
```

The `model`, `grader`, and `dictionary` properties are required. The rest are optional.

## Provenance

A score links to the model and grader records that produced it. The model record carries the name, provider, and variant. The grader record carries the name, stub, commit, branch, version, and uncommitted flag. The service stores the records and the link, as defined in the [Schema Design](schema.md).
