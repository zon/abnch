Confirm test role access over the service API

The service already authorized every API request after authentication, so test
read operations were allowed for the Reader, Service, and Admin roles and test
writes were allowed for Admin only with no change. This step confirms the
contract for the test resource and closes its authorization coverage.

Tests cover listing and getting a test answering `200` for the Reader, Service,
and Admin roles, a denied read answering `403` without reaching the store for
the Agent and Grader roles, and an update, a create, and a move answering `403`
with the stored test unchanged for every role but Admin. Existing tests already
cover an Agent read answering `403` and an Admin create and move succeeding.

tests-and-data-0jRgccD
