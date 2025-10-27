.PHONY: all build clean web server install

all: build

web:
	cd web && npm run build

server: web
	go build -o share .

build: server

install: build
	go install

clean:
	rm -f share
	rm -rf web/dist
