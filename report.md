Preserve models and tasks when `set config --url` updates the URL

`abnch set config --url` updates the url in `.abnch/config.yaml` while leaving every existing model and task untouched. Expand the `set config` command test to cover multiple models and tasks, including model variants and nested agent and grader settings, and assert they survive the URL update unchanged.

repo-config-6JURWqs
