# Container Standard

## Common

* Pin base images at the latest available version
* Pin dependencies added outside distribution packaging
* Check online for the latest version before pinning
* Use multi-stage containers to keep build resources separate
* Prefer smaller base images
* Prefer Containerfiles over Dockerfiles
* Prefer Podman over Docker

### Sharing

Apps in the same repo share a container when:

* Both apps share a base image
* Both apps share dependencies

### Dividing

Apps get their own containers when:

* The app's layer is a significant portion of the image size

## Continuous Delivery

Follow this process:

* Build
* Scan with Trivy
* Stop on high or critical vulnerabilities
* Push
