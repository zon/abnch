Ship the `abnchd` Helm chart

Add `deploy/helm/abnchd`, an application chart that installs the service into
a Kubernetes cluster. Its values hold the service image repository, tag, pull
policy, replica count, and Service type and port. A Deployment runs the service
image, taking the tag from the `image.tag` value or the chart app version when
the value is empty, and a ClusterIP Service selects the Deployment's pods and
forwards port 80 to the container's named `http` port. Shared helpers set the
release-scoped name, common labels, and selector labels.

Tests in `deploy/helm` render the chart with `helm template`. They cover the
chart manifest, the Deployment and Service, the default service image and a
tag override, the Service selector matching the pod labels and reaching the
container port, and a clean `helm lint`. The architecture document records the
new `deploy/helm` component.

helm-chart-CcUmvSN
