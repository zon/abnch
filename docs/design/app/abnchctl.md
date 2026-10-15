# Dev Tooling Design

## Common Options

* --config - string - abnchd config path. Defaults to `<repo>/.abnchd/config.yaml`

## `abnchctl set local-auth`

Generates local `abnchd` auth config and the `abnch` [CLI config](abnch-config.md) that points at the local service.

* --port - int - local service port. Defaults to `8080`

## `abnchctl check config`

Confirms local `abnchd` config is ready. At least one oidc or oauth entry and postgres connection config should be required.