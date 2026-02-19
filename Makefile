.PHONY: build run test templ clean lint dev migrate verify install

build: templ
	go build -o bin/veristoretools3 ./cmd/server
	go build -o bin/migrate ./cmd/migrate

run: templ
	go run ./cmd/server

test:
	go test ./... -v

templ:
	templ generate

clean:
	rm -rf bin/

lint:
	golangci-lint run ./...

dev: templ
	go run ./cmd/server

migrate:
	go run ./cmd/migrate

verify:
	go run ./cmd/migrate verify

install:
	./deploy/install.sh
