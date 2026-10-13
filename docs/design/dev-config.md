# Dev Config Design

Developers run the service locally against Postgres. The [Just](https://just.systems/) recipe `just config` collects the [Postgres](service-config.md#postgres) connection details and writes a local [service config](service-config.md) file. The file stays out of git.

## Local Service Config

`just config` writes `service-config.local.yaml` in the repo root. The file is a [service config](service-config.md) and holds only the values a local run needs. Git ignores it.

## `just config`

The recipe prompts for each [Postgres](service-config.md#postgres) field:

* host - database host
* port - database port
* dbname - database name
* user - database user
* password - database password
* sslmode - connection ssl mode, default `disable`
* timeZone - session time zone, default `UTC`

The password prompt hides input. A value already in `service-config.local.yaml` becomes the prompt default, so a later run changes one value without retyping the rest. The recipe writes the file after the last prompt and reports its path.

## Running the Service

A local run points the service at `service-config.local.yaml`. The service then connects to Postgres with the collected details. See [Service Config Design](service-config.md).
