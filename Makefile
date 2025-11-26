.PHONY: all build clean web server install migrate migrate\:create

# Paths/commands
DB_PATH ?= relay_logs.db
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

# Ensure migrate CLI is available (installs with sqlite and postgres support if missing)
define ensure_migrate
	@command -v migrate >/dev/null || (echo "Installing golang-migrate CLI..." && go install -tags 'postgres,sqlite' github.com/golang-migrate/migrate/v4/cmd/migrate@latest)
endef

# Apply migrations to the configured database
# If DATABASE_URL is set (in .env), use PostgreSQL; otherwise use SQLite
migrate:
	$(call ensure_migrate)
	@if [ -f .env ]; then \
		set -a && . ./.env && set +a && \
		if [ -n "$$DATABASE_URL" ]; then \
			echo "Applying PostgreSQL migrations..."; \
			migrate -path $(MIGRATIONS_DIR)/postgres -database "$$DATABASE_URL" up; \
		else \
			mkdir -p $(dir $(DB_PATH)); \
			echo "Applying SQLite migrations to $(abspath $(DB_PATH))"; \
			migrate -path $(MIGRATIONS_DIR)/sqlite -database "sqlite://$(abspath $(DB_PATH))" up; \
		fi; \
	else \
		mkdir -p $(dir $(DB_PATH)); \
		echo "Applying SQLite migrations to $(abspath $(DB_PATH))"; \
		migrate -path $(MIGRATIONS_DIR)/sqlite -database "sqlite://$(abspath $(DB_PATH))" up; \
	fi

# Create a new timestamped migration: make migrate:create name=add_table
# Creates migrations in both sqlite/ and postgres/ directories
migrate\:create:
	@test -n "$(name)" || (echo "Usage: make migrate:create name=<migration_name>" && exit 1)
	$(call ensure_migrate)
	@echo "Creating SQLite migration..."
	@migrate create -ext sql -dir $(MIGRATIONS_DIR)/sqlite -seq $(name)
	@echo "Creating PostgreSQL migration..."
	@migrate create -ext sql -dir $(MIGRATIONS_DIR)/postgres -seq $(name)
	@echo "Created migration files. Remember to customize them for each database!"
