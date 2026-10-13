# Service Config Design

The service reads its configuration from a YAML file. A deployment provides the file. Secret handling belongs to [Deployment Design](deployment.md).

## OIDC Integration

Human users sign in through an OpenID Connect provider. The service uses the authorization code flow, then grants the Reader or Admin role from a mapped claim. See [Auth Design](auth.md).

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

A deployment reads the password from a secret. Local development collects these values with `just config`. See [Dev Config Design](dev-config.md).
