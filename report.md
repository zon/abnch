`abnch score` accepts `--model` to override the task's default model and
`--local` to select a local run, and fails non-zero when `--model` names a
model the config does not hold.

The command resolves the model from the flag first, then the task's default
model, against the CLI config. A `--model` that names a model the config does
not hold fails non-zero with the model named on stderr before the grader builds
or runs, so it records no score. `--local` is accepted as the only run mode,
matching `abnch run`. The score command reuses the run command's
`resolveRunModel`, so the override and the unknown-model failure share one path.

Tests cover an unknown `--model` failing with the model named and no build, grade
run, or score, the non-zero exit and stderr for that failure, and `--local`
accepted and recording a score. Existing tests already cover the model flag
overriding the task's default model.

graders-and-scores-7o2IwuT
