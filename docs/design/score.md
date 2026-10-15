# Score Design

Grading executes a task's grader against a result and records a [score](schema.md).

## Models

A model is a provider, a name, and a variant. Models are recorded in the [CLI config](app/abnch-config.md). A grade run passes a model to the grader, as a [run](run.md) passes one to an agent.

## Graders

A task records a grader in the [CLI config](app/abnch-config.md). A grader carries a version command, a build command, and a run command, like an agent. The commands run in the repo under the working directory.

* The version command prints the grader's version
* The build command prepares the grader to run
* The run command produces a score

## Grader Interface

The run command reads a grader input as JSON on stdin. The input carries the model config, the datum that produced the result, and the result.

```json
{
  "model": { "name": "claude-sonnet", "provider": "anthropic", "variant": "high" },
  "data": { "prompt": "hi" },
  "result": { "answer": "hi" }
}
```

The run command writes its logs to stdout, then the abnch marker, then a score output as JSON.

```
grading the answer
__abnch__
{ "score": { "correctness": 1 }, "inputTokens": 12, "outputTokens": 3, "tokenCost": 0.01 }
```

The marker is the same marker a [run](run.md) uses. Text before it is the grade run's logs. The JSON after it is the score output.

## Score Output

* score - the score as a key-number dictionary
* inputTokens - optional input token count
* outputTokens - optional output token count
* tokenCost - optional token cost

## Local Runs

`abnch score --local` runs a grader on the caller's machine. The CLI builds the grader once, runs it against the result, measures the run's duration, and records the score. A deployment runs graders as Kubernetes workflows instead. See [Deployment Design](deployment.md).
