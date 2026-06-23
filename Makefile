.PHONY: build test clean

BINARY := doxygen-mcp

build:
	go build ./cmd/doxygen-mcp

test:
	go test ./...

clean:
	rm -f $(BINARY)
