# Dev Config Design

Developers run the service locally against Postgres. The [Just](https://just.systems/) recipe `just config` collects the [Postgres](service-config.md#postgres) connection details, generates a dev signing key, and writes a local [service config](service-config.md) file. The file stays out of git.

## Local Service Config

`just config` writes `service-config.local.yaml` in the repo root. The file is a [service config](service-config.md) and holds only the values a local run needs. Git ignores it.

## `just config`

The recipe takes an optional port for the local service and defaults to `8080`. The port sets the local service URL, `http://localhost:<port>`.

The recipe prompts for each [Postgres](service-config.md#postgres) field:

* host - database host
* port - database port
* dbname - database name
* user - database user
* password - database password
* sslmode - connection ssl mode, default `disable`
* timeZone - session time zone, default `UTC`

The password prompt hides input. A value already in `service-config.local.yaml` becomes the prompt default, so a later run changes one value without retyping the rest. The recipe writes the file after the last prompt and reports its path.

## Local Service Runtime Auth

`just config` generates a dev signing key, a dev issuer, and local CLI credentials, so a client can authenticate to a local service runtime.

* It generates an asymmetric key pair and writes the private key to `dev-issuer.key` in the repo root. Git ignores the file.
* It adds the dev issuer to `oauth.issuers` in `service-config.local.yaml`. The issuer URL is the local service URL plus `/dev`, the audience is `abnch`, and the role claim is `sub`.
* The role map grants `Admin` to `local-cli` and `Service` to `local-runtime`.
* The local service serves the dev issuer's OIDC configuration and JWKS.
* It writes a local CLI credential to `~/.abnch/credentials.yaml`, keyed by the local service URL.

`abnch` reads the local credential and presents it to the local service. It mints a Service token from the dev key for a runtime it starts and passes the token in the runtime's environment. See [Auth Design](auth.md).

## Running the Service

A local run points the service at `service-config.local.yaml`. The service then connects to Postgres with the collected details. See [Service Config Design](service-config.md).
