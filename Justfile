# Set up a local service: write the local auth config, dev key, and CLI
# credential. Write the Postgres details by hand first.
config port="8080":
	@go run ./cmd/abnchctl set local-auth --port {{port}}
	@go run ./cmd/abnchctl check config

# List available recipes
default:
	@just --list
