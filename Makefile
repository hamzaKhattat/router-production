.PHONY: build clean run install

BINARY=router

build:
   go mod tidy
   go build -o bin/$(BINARY) cmd/router/main.go

clean:
   rm -rf bin/

run: build
   ./bin/$(BINARY) server

install: build
   sudo cp bin/$(BINARY) /usr/local/bin/
   sudo chmod +x /usr/local/bin/$(BINARY)

test:
   go test -v ./...
