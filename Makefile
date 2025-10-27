.PHONY: all build clean web server install

LDFLAGS ?=

all: build

web:
	cd web && npm run build

server: web
	go build -ldflags "$(LDFLAGS)" -o share .

build: server

install: build
	go install

clean:
	rm -f share
	rm -rf web/dist
