# Dev Config Design

Developers run the service locally against Postgres. The [Just](https://just.systems/) recipe `just config` writes the local service URL and the local auth config into a local [service config](app/abnchd-config.md) file. A human or an agent writes the [Postgres](app/abnchd-config.md#postgres) connection details. The file stays out of git.

## Local Service Config

`just config` writes `.abnchd/config.yaml` under the repo root. The file is a [service config](app/abnchd-config.md) and holds only the values a local run needs. The dev signing key sits beside it in the same directory, so the config and the key that signs its issuer move together. Git ignores the directory.

A human or an agent writes the [Postgres](app/abnchd-config.md#postgres) block into the file. `just config` leaves that block unchanged.

## `just config`

The recipe takes an optional port for the local service and defaults to `8080`. The port sets the local service URL, `http://localhost:<port>`.

`just config` writes the local service URL and the [local service runtime auth](#local-service-runtime-auth), then reports the config path.

It also writes the local service URL to the [CLI config](app/abnch-config.md) at `.abnch/config.yaml` under the repo root, so the `abnch` CLI reaches the local service without `--url`.

## Local Service Runtime Auth

`just config` generates a dev signing key, a dev issuer, and local CLI credentials, so a client can authenticate to a local service runtime.

* It generates an asymmetric key pair and writes the private key to `dev-issuer.key` beside the config, in the `.abnchd` directory. Git ignores the directory.
* It adds the dev issuer to `oauth.issuers` in `.abnchd/config.yaml`. The issuer URL is the local service URL plus `/dev`, the audience is `abnch`, and the role claim is `sub`.
* The role map grants `Admin` to `local-cli`.
* The local service serves the dev issuer's OIDC configuration and JWKS.
* It writes a local CLI credential to `~/.abnch/credentials.yaml`, keyed by the local service URL.

`abnch` reads the local credential and presents it to the local service. See [Auth Design](auth.md).

## Running the Service

`just service` starts `abnchd` against `.abnchd/config.yaml`. Run `just config` first. The service connects to Postgres with the details in the file. See [Service Config Design](app/abnchd-config.md).

Run the service through [Air](https://github.com/air-verse/air) for live reload. `just service` runs `air`, which builds `cmd/abnchd` into `tmp/` and restarts the binary on a Go source change. The build and its output stay out of git. Install Air with `go install github.com/air-verse/air@latest`.
