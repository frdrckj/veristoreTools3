.PHONY: build run test templ clean lint dev

build: templ
	go build -o bin/veristoretools3 ./cmd/server

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
