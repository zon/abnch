# Container Specification

## Purpose

Package and deliver the repo's apps as container images. Images follow the [Container Standard](../../docs/zpecs/containers.md). The service image runs `abnchd` from a service config, as the [Deployment Design](../../docs/design/deployment.md) describes. Chart values, secrets, and cluster resources are out of scope.

## Requirements

### Requirement: Image Set

The repo SHALL build one image for each app it delivers. Apps SHALL share an image when they share a base image and dependencies, and SHALL get their own image when one app's layer is a significant portion of the image size.

#### Scenario: Service image

- GIVEN the repo
- WHEN the service image builds
- THEN it holds `abnchd`

### Requirement: Multi-Stage Build

An image SHALL build from a Containerfile with a build stage and a runtime stage, so build tools stay out of the runtime image.

#### Scenario: Runtime holds the app

- GIVEN a Containerfile for an app
- WHEN its image builds
- THEN the build stage compiles the app
- AND the runtime stage holds only the app, its runtime dependencies, and no build tools

### Requirement: Pinned Inputs

An image SHALL pin its base image to a version and SHALL pin dependencies added outside distribution packaging. A base image SHALL be the latest available version when pinned.

#### Scenario: Base image pinned

- GIVEN a Containerfile
- WHEN it names its base image
- THEN the base names a pinned version
- AND it does not name a moving tag such as `latest`

#### Scenario: Dependencies pinned

- GIVEN an image whose dependencies come from outside distribution packaging
- WHEN the image builds
- THEN those dependencies are pinned to a version

### Requirement: Service Run

The service image SHALL run `abnchd` with the service config path the caller supplies.

#### Scenario: Start from a config

- GIVEN a service config file
- WHEN the service image starts with that file's path
- THEN `abnchd` runs with that config
- AND the service serves its API

#### Scenario: Config missing

- GIVEN no service config path
- WHEN the service image starts
- THEN `abnchd` fails
- AND it reports the error

### Requirement: Image Scan

An image SHALL scan clean of high and critical vulnerabilities before it is pushed.

#### Scenario: Clean image

- GIVEN an image with no high or critical vulnerability
- WHEN the scan runs
- THEN the scan passes
- AND the image may be pushed

#### Scenario: Vulnerable image

- GIVEN an image with a high or critical vulnerability
- WHEN the scan runs
- THEN the scan fails
- AND the image is not pushed

### Requirement: Delivery Pipeline

The repo SHALL deliver images with a pipeline that builds, scans, and pushes, in that order.

#### Scenario: Deliver an image

- GIVEN a change to the repo
- WHEN the pipeline runs
- THEN it builds the image
- AND it scans the image
- AND it pushes the image only when the scan passes
