# Milestones

## Repo Config

* A new user can set up a fresh repo without editing files by hand
* `abnch set config --url <url>` creates `.abnch/config.yaml` with the service URL
* `abnch get config` prints the effective config
* Config updates for models, tasks, agents, and graders keep the service URL
* A command that needs the service URL and finds none says how to set it
