# Dev Tooling Design

## Common Options

* --config - string - abnchd config path. Defaults to `<repo>/.abnchd/config.yaml`

## `abnchctl set local-auth`

Generates local `abnchd` and `abnch` auth config.

* --port - int - local service port. Defaults to `8080`

## `abnchctl check config`

Confirms local `abnchd` config is ready. At least one oidc or oauth entry and postgres connection config should be required.