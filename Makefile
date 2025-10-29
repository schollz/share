.PHONY: all build clean web server install

LDFLAGS ?=

all: build
	@echo "Build completed at $(shell date)"

web:
	cd web && npm run build
	touch web/dist/.keep

server: web
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS) -extldflags '-static'" -o share .

build: server

install: build
	go install

clean:
	rm -f share
	rm -rf web/dist
