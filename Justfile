# Set up a local service: collect the Postgres details, generate a dev key, and
# write the local service config and CLI credential.
config port="8080":
	@go run ./cmd/abnchctl set local-auth --port {{port}}
	@go run ./cmd/abnchctl check config

# List available recipes
default:
	@just --list
