.PHONY: all build clean web server install

LDFLAGS ?=

all: build
	@echo "Build completed at $(shell date)"

web:
	cd web && npm run build
	touch web/dist/.keep

server: web
	go build -ldflags "$(LDFLAGS)" -o share .

build: server

install: build
	go install

clean:
	rm -f share
	rm -rf web/dist
