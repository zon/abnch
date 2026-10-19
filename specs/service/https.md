# HTTPS Service API Specification

## Purpose

Serve the service API over HTTPS, with `abnchd` terminating TLS from a certificate its deployment supplies. Payload encryption, encryption at rest, and mutual TLS are out of scope. Error shapes and authorization follow the [Repo Service API](repo.md).

## Requirements

### Requirement: HTTPS Serving

When the service config names a certificate and key, the service SHALL serve every API request over HTTPS on its listen address. It SHALL NOT serve the API over plaintext on another address. It SHALL use TLS 1.2 or newer.

#### Scenario: HTTPS request

- GIVEN a service config with a certificate, key, and listen address
- WHEN a client sends an HTTPS request
- THEN the service serves the request over TLS
- AND the response comes over the same TLS connection

#### Scenario: Plaintext request

- GIVEN a service serving HTTPS on an address
- WHEN a client sends a plaintext HTTP request to that address
- THEN the request fails
- AND the service performs no operation

### Requirement: TLS Config

The service SHALL read the certificate from `tls.certFile` and the private key from `tls.keyFile`. The service SHALL fail to start when either file is missing or unreadable, or when the pair is not a valid certificate and key.

An `https` URL without a TLS block SHALL fail to start. An `http` URL with a TLS block SHALL fail to start.

#### Scenario: Missing certificate

- GIVEN a service config naming a certificate file that does not exist
- WHEN the service starts
- THEN it fails
- AND it reports the missing file

#### Scenario: Invalid pair

- GIVEN a certificate and a key that do not pair
- WHEN the service starts
- THEN it fails
- AND it reports the invalid pair

#### Scenario: HTTPS URL without TLS

- GIVEN a service config whose URL uses `https` and has no TLS block
- WHEN the service starts
- THEN it fails
- AND it reports that the URL needs TLS config

#### Scenario: HTTP URL with TLS

- GIVEN a service config whose URL uses `http` and has a TLS block
- WHEN the service starts
- THEN it fails
- AND it reports that the URL must use `https`

### Requirement: Listen Address

The service SHALL bind the address in `listen`. When `listen` is absent, it SHALL bind the host and port in `url`.

The service SHALL use `url` as its public base URL, wherever it addresses itself, including the OIDC redirect and the dev issuer URLs.

#### Scenario: Separate listen and URL

- GIVEN a service config with URL `https://bench.example.com` and listen `0.0.0.0:8443`
- WHEN the service starts
- THEN it listens on `0.0.0.0:8443`
- AND its self-references use `https://bench.example.com`

#### Scenario: Listen defaults to URL

- GIVEN a service config with URL `http://localhost:8080` and no listen address
- WHEN the service starts
- THEN it listens on `localhost:8080`

### Requirement: Certificate Renewal

The service SHALL serve a renewed certificate without a process restart. When the files at `tls.certFile` and `tls.keyFile` become a valid pair, the service SHALL use the new pair for new connections.

#### Scenario: Renewed certificate

- GIVEN a running service serving certificate A
- WHEN the configured files are replaced with a valid certificate B and its key
- THEN new TLS connections use certificate B
- AND the service does not restart

### Requirement: Plaintext Development

A service config with no TLS block SHALL serve HTTP on `listen`. A local service SHALL use this mode on loopback.

#### Scenario: Local service

- GIVEN a service config with URL `http://localhost:8080` and no TLS block
- WHEN the service starts
- THEN it serves HTTP on `localhost:8080`
