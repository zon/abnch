# Dev Config Design

Developers run the service locally against Postgres. The [Just](https://just.systems/) recipe `just config` writes the local service URL and the local auth config into a local [service config](app/abnchd-config.md) file. A human or an agent writes the [Postgres](app/abnchd-config.md#postgres) connection details. The file stays out of git.

## Local Service Config

`just config` writes `.abnchd/config.yaml` under the repo root. The file is a [service config](app/abnchd-config.md) and holds only the values a local run needs. The dev signing key sits beside it in the same directory, so the config and the key that signs its issuer move together. Git ignores the directory.

A human or an agent writes the [Postgres](app/abnchd-config.md#postgres) block into the file. `just config` leaves that block unchanged.

## `just config`

The recipe takes an optional port for the local service and defaults to `8080`. The port sets the local service URL, `http://localhost:<port>`.

`just config` writes the local service URL and the [local service runtime auth](#local-service-runtime-auth), then reports the config path.

## Local Service Runtime Auth

`just config` generates a dev signing key, a dev issuer, and local CLI credentials, so a client can authenticate to a local service runtime.

* It generates an asymmetric key pair and writes the private key to `dev-issuer.key` beside the config, in the `.abnchd` directory. Git ignores the directory.
* It adds the dev issuer to `oauth.issuers` in `.abnchd/config.yaml`. The issuer URL is the local service URL plus `/dev`, the audience is `abnch`, and the role claim is `sub`.
* The role map grants `Admin` to `local-cli` and `Service` to `local-runtime`.
* The local service serves the dev issuer's OIDC configuration and JWKS.
* It writes a local CLI credential to `~/.abnch/credentials.yaml`, keyed by the local service URL.

`abnch` reads the local credential and presents it to the local service. It mints a Service token from the dev key for a runtime it starts and passes the token in the runtime's environment. See [Auth Design](auth.md).

## Running the Service

A local run points the service at `.abnchd/config.yaml`. The service then connects to Postgres with the details in the file. See [Service Config Design](app/abnchd-config.md).
