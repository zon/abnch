Confirm a test write to a missing task answers 404

`PUT /repos/{org}/{repo}/tasks/{task}/tests/{stub}` already checked that the
repo and task exist before it wrote anything, so a write to a missing task
answered `404` and created no task or test. This step confirms the contract and
names its test after it.

Tests cover a write to a task that does not exist answering `404` and leaving
the store with no task and no test.

tests-and-data-9m3iFdp
