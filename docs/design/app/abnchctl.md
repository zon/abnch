# Dev Tooling Design

## Common Options

* --config - string - abnchd config path. Defaults to `<repo>/.abnchd/config.yaml`

## `abnchctl set local-auth`

Generates local `abnchd` auth config and the `abnch` [CLI config](abnch-config.md) that points at the local service.

* --port - int - local service port. Defaults to `8080`

## `abnchctl set ralph`

Publishes the local `.abnchd` and `.abnch` files to the Ralph namespace, so a remote workflow that mounts them reads the same config a local service uses. It writes the local service config and the dev key beside it to the `abnchd-config` Secret, and the [CLI config](abnch-config.md) to the `abnch-config` ConfigMap. The repo's `.ralph/config.yaml` declares the mounts.

The namespace comes from `workflow.namespace` in `.ralph/config.yaml`. It fails when the file sets none.

* --context - string - Kubernetes context. Overrides `workflow.context`
* --namespace - string - Kubernetes namespace. Overrides `workflow.namespace`

## `abnchctl check config`

Confirms local `abnchd` config is ready. At least one oidc or oauth entry and postgres connection config should be required.