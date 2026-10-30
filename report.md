Return the model and agent linked by a result write

`POST /repos/{org}/{repo}/results` already accepted nested model and agent
records, created or reused them through the store, linked them to the result,
and returned them when the result was read. This step closes the coverage the
item calls for.

Added service tests that post a result carrying both records and read it back,
and that post two results sharing one model and one agent and read both back,
asserting that each read returns the linked records and that the store holds a
single record of each.

result-provenance-16UDRQO
