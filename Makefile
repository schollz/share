.PHONY: all build clean web server install migrate migrate\:create

# Paths/commands
MIGRATIONS_DIR ?= migrations

LDFLAGS ?=

all: build
	@echo "Build completed at $(shell date)"

web/node_modules:
	cd web && npm install

web: web/node_modules
	cd web && npm run build
	touch web/dist/.keep

server: web
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS) -extldflags '-static'" -o e2ecp .

build: server

install: build
	go install

clean:
	rm -f e2ecp
	rm -rf web/dist

test: all
	go test -v ./...
	cd tests && ./run-tests.sh

# Ensure migrate CLI is available (installs with postgres support if missing)
define ensure_migrate
	@command -v migrate >/dev/null || (echo "Installing golang-migrate CLI..." && go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest)
endef

# Apply migrations to the configured database
# Requires DATABASE_URL to be set (from the environment or .env)
migrate:
	$(call ensure_migrate)
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; \
	if [ -z "$$DATABASE_URL" ]; then \
		echo "DATABASE_URL is not set; cannot run migrations."; \
		exit 1; \
	fi; \
	echo "Applying PostgreSQL migrations..."; \
	migrate -path $(MIGRATIONS_DIR)/postgres -database "$$DATABASE_URL" up

# Create a new timestamped migration: make migrate:create name=add_table (postgres only)
migrate\:create:
	@test -n "$(name)" || (echo "Usage: make migrate:create name=<migration_name>" && exit 1)
	$(call ensure_migrate)
	@echo "Creating PostgreSQL migration..."
	@migrate create -ext sql -dir $(MIGRATIONS_DIR)/postgres -seq $(name)
	@echo "Created PostgreSQL migration files."
