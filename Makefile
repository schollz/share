.PHONY: all build clean web server install

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
