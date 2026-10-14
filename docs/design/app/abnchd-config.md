# Service Config Design

The [service app](abnchd.md) reads its configuration from a YAML file. A deployment provides the file. A local run reads a file that `just config` wrote. See [Dev Config Design](../dev-config.md). Secret handling belongs to [Deployment Design](../deployment.md).

## Service URL

`url` is the service's own base URL. The service listens on its host and port, and uses the URL wherever it addresses itself: the OIDC redirect, and the dev issuer's OIDC configuration and JWKS.

```yaml
url: string - service base URL, for example `http://localhost:8080`
```

## Postgres

The service stores domain data in Postgres through the [Gorm](https://gorm.io) Postgres driver. The service joins the values below into the driver's DSN. The keys match the DSN keywords, so `timeZone` maps to the driver's capitalized `TimeZone` keyword.

```yaml
postgres:
  host: string - database host
  port: int - database port
  dbname: string - database name
  user: string - database user
  password: string - database password. A deployment reads it from a secret
  sslmode: string - connection ssl mode
  timeZone: string - session time zone, for example `UTC`
```

A deployment reads the password from a secret. Local development writes these values by hand or with an agent. See [Dev Config Design](../dev-config.md).

## OIDC Integration

Human users sign in through an OpenID Connect provider. The service uses the authorization code flow, then grants the Reader or Admin role from a mapped claim. See [Auth Design](../auth.md).

```yaml
oidc:
  issuer: string - OIDC issuer URL. The service discovers endpoints from it
  clientId: string - client registered with the provider
  clientSecret: string - client secret. A deployment reads it from a secret
  redirectUrl: string - service URL the provider calls back after sign in
  scopes: [string] - scopes requested at sign in
  roleClaim: string - claim that carries role values
  roleMap:
    <claim value>: <Reader|Admin> - role granted for the claim value
```

A claim value with no mapping grants no role.

## Workload Authentication

Non-human callers authenticate with an OAuth token, without a sign-in. Workloads are Kubernetes service accounts, GitHub workflows, and local clients of a dev issuer. See [Auth Design](../auth.md).

A workload token is an OIDC JWT. The service discovers each trusted issuer's keys from its OIDC configuration, then checks the signature, the audience, and the expiry.

```yaml
oauth:
  issuers:
    - issuer: string - OIDC issuer URL. The service discovers endpoints from it
      audience: string - audience a token must carry to be accepted
      roleClaim: string - claim that carries the caller identity
      roleMap:
        <claim value>: <Service|Agent|Grader> - role granted for the claim value
```

The service rejects a token with an unknown issuer, a wrong audience, a bad signature, or an expiry in the past.

A Kubernetes service account token carries the `sub` claim `system:serviceaccount:<namespace>:<name>`. A GitHub workflow token carries claims such as `repository` and `ref`.

A dev issuer token is minted by the CLI and signed with a dev key. A local service config trusts the dev issuer that signs it, and the service serves that issuer's public half from its JWKS. The dev key sits beside the config in the `.abnchd` directory. See [Dev Config Design](../dev-config.md).

## Local Development

A local run reads a config that `just config` writes. Use the [dev tooling](abnchctl.md) to set it up:

* Write the Postgres connection details into `.abnchd/config.yaml` by hand or with an agent.
* Run `abnchctl set local-auth` to configure local auth. It writes the dev issuer and the local credential.
* Run `abnchctl check config` to confirm the config files. It expects the auth and Postgres config to already be present.
