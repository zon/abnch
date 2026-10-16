engine := "podman"
image := "abnchd:local"

# Set up a local service: write the local auth config, dev key, CLI config, and
# CLI credential. Write the Postgres details by hand first.
config port="8080":
	@go run ./cmd/abnchctl set local-auth --port {{port}}
	@go run ./cmd/abnchctl check config

# Start the local service from the config `just config` wrote, with live reload
service:
	@air

# Build the service image and scan it for high and critical vulnerabilities
container:
	{{engine}} build -f deploy/containers/Containerfile -t {{image}} .
	trivy image --severity HIGH,CRITICAL --exit-code 1 {{image}}

# List available recipes
default:
	@just --list
