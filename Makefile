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

# Ensure migrate CLI is available (installs with sqlite support if missing)
define ensure_migrate
	@command -v migrate >/dev/null || (echo "Installing golang-migrate CLI..." && go install -tags 'sqlite' github.com/golang-migrate/migrate/v4/cmd/migrate@latest)
endef

# Apply migrations to the configured database
migrate:
	$(call ensure_migrate)
	@mkdir -p $(dir $(DB_PATH))
	@echo "Applying migrations to $(abspath $(DB_PATH))"
	@migrate -path $(MIGRATIONS_DIR) -database "sqlite://$(abspath $(DB_PATH))" up

# Create a new timestamped migration: make migrate:create name=add_table
migrate\:create:
	@test -n "$(name)" || (echo "Usage: make migrate:create name=<migration_name>" && exit 1)
	$(call ensure_migrate)
	@migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)
