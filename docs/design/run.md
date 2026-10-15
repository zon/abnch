# Run Design

A run executes a task's agent against a datum and records a [result](schema.md).

## Models

A model is a provider, a name, and a variant. Models are recorded in the [CLI config](app/abnch-config.md). A run passes a model to the agent.

## Agents

A task records an agent in the [CLI config](app/abnch-config.md). An agent carries a version command, a build command, and a run command. The commands run in the repo under the working directory.

* The version command prints the agent's version
* The build command prepares the agent to run
* The run command produces a result

## Agent Interface

The run command reads a run input as JSON on stdin. The input carries the model config and the datum.

```json
{
  "model": { "name": "claude-sonnet", "provider": "anthropic", "variant": "high" },
  "data": { "prompt": "hi" }
}
```

The run command writes its logs to stdout, then the abnch marker, then a run output as JSON.

```
resolving the prompt
__abnch__
{ "json": { "answer": "hi" }, "inputTokens": 12, "outputTokens": 3, "tokenCost": 0.01 }
```

The abnch marker is a line carrying exactly `__abnch__`. Text before it is the run's logs. The JSON after it is the run output.

## Run Output

* json - the result the agent produced
* inputTokens - optional input token count
* outputTokens - optional output token count
* tokenCost - optional token cost

## Local Runs

`abnch run --local` runs an agent on the caller's machine. The CLI builds the agent once, runs it once per datum, measures each run's duration, and records the working directory's Git branch as the result's local branch.

A deployment runs agents as Kubernetes workflows instead. See [Deployment Design](deployment.md).
