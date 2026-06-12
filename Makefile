.PHONY: build init sync status watch ssh stop clean

# Build the Docker image
build:
	docker compose build

# Initialize the documents directory
init:
	docker compose run --rm remarker init

# One-time sync
sync:
	docker compose run --rm remarker sync

# Show pending changes
status:
	docker compose run --rm remarker status

# Watch mode (background)
watch:
	docker compose up

# SSH shell
ssh:
	docker compose run --rm remarker ssh

# Stop all containers
stop:
	docker compose down

# Remove containers, volumes, and images
clean:
	docker compose down -v --rmi local
