Apply a semver minor bump to the app version

The shared app version moves from 1.1.0 to 1.2.0 in
`internal/version/VERSION`. The Helm chart's `version` and `appVersion` follow,
keeping the version the apps report, the default image tag, and the chart in
step.

Add a test in `internal/version` that holds the version at or past 1.2.0, so a
later change cannot silently revert it below the Repo Config release.

Ran the full Go test suite.

repo-config-5bDJnLF
