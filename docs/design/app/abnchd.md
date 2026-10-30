# Service App Design

## `abnchd <config>`

Starts the service with a config file. It does not create or change the database tables.

## `abnchd migrate <config>`

Creates or updates the database tables the service needs, then exits. It reads the same config file as the service and does not serve.

## `abnchd --version`

Prints the semantic version the repo's apps share and exits, without reading a config.
