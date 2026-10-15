Cover test create and update over the service API

`PUT /repos/{org}/{repo}/tasks/{task}/tests/{stub}` creates a missing test and
returns `201`, merges the body into an existing test and returns `200`, and
replaces the schema when the body carries one. The service already served this
route through the test write path, so this step confirms the contract and closes
its test coverage.

Tests cover a create then an update of the same test answering `201` then `200`,
an update that omits the schema keeping the stored one, an update that carries an
empty schema clearing it, and an update that merges the name and description
while keeping the version.

tests-and-data-DMabPbz
