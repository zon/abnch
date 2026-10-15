# Set up a local service: write the local auth config, dev key, CLI config, and
# CLI credential. Write the Postgres details by hand first.
config port="8080":
	@go run ./cmd/abnchctl set local-auth --port {{port}}
	@go run ./cmd/abnchctl check config

# Start the local service from the config `just config` wrote, with live reload
service:
	@air

# List available recipes
default:
	@just --list
