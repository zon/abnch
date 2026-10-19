# HTTPS Design

The service serves its API over HTTPS. TLS starts at abnchd. The process reads the certificate and key, then accepts TLS itself, rather than leaving TLS to an ingress or a proxy in front of it. A deployment issues the certificate with cert-manager and mounts it into the pod.

HTTPS protects traffic between a client and abnchd. It does not encrypt data in Postgres or hide data from the service. Payload encryption, encryption at rest, and mutual TLS are out of scope. The Postgres connection has its own `sslmode` setting, separate from the client-facing HTTPS.

Clients use the service URL and verify the certificate against their trust store. A deployment with a private CA distributes the CA to its clients. A public CA needs no distribution.

## Service Config

`url` is the service's public base URL. The service uses it for self-references, such as the OIDC redirect and the dev issuer URLs.

`listen` is the TCP address the service binds. It defaults to the host and port in `url`, so a local config can omit it. A deployment sets it to the pod's bind address, so the public URL can name the service's external host.

`tls.certFile` and `tls.keyFile` name the certificate and key. When both are set, the service serves HTTPS on `listen`.

An `https` URL needs a TLS block. An `http` URL needs no TLS block. A config that breaks this rule fails to start, so the service never serves a URL whose scheme it does not honor.

A service without a TLS block serves HTTP on `listen`. Local development uses this mode on loopback.

## Certificate Renewal

cert-manager renews a certificate by writing a new value to the certificate's Secret. The service reads the files at startup and reloads them when they change. A valid replacement takes effect for new connections without a pod restart.

## Helm Chart

The chart requires cert-manager. It creates the service Certificate and, by default, a private CA that signs it. The Deployment mounts the Certificate Secret and starts abnchd over HTTPS.

The chart creates these cert-manager resources by default:

* a self-signed Issuer
* a CA Certificate
* an Issuer backed by the CA
* the service Certificate

A deployment with its own issuer sets `tls.createIssuer` to false and names the issuer in `tls.issuerRef`. The chart fails to render when that name is missing.

The service Certificate covers `tls.dnsNames`. The default is the Service's cluster DNS name. A deployment that serves clients outside the cluster sets the public host names in `tls.dnsNames` and the same host in `config.url`.

The CA Certificate's Secret holds the CA certificate. A deployment distributes it to clients that must trust the private CA.

The Service forwards its HTTPS port to the container's HTTPS port. The chart sets `config.listen` to `0.0.0.0:8443` and `config.url` to the deployment's public URL.
