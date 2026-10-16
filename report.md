Pin the chart's default service image to the repo version

The chart's image values already default to the service image, with the tag
falling back to the chart appVersion. Record on the chart that appVersion is
the repo version, and add tests that hold the default to it, so a caller
installs the chart and runs the image the apps report.

Tests in `deploy/helm` render the chart with `helm template`. They cover the
default container image naming `ghcr.io/zon/abnch/abnchd` at
`internal/version.Version`, the Chart.yaml appVersion equalling that version,
the values naming the service image, and an `image.repository` override
reaching the rendered Deployment.

helm-chart-86NEuMM
