.PHONY: build test docker-build docker-test clean

IMAGE := doxygen-mcp
TEST_IMAGE := doxygen-mcp-test
BINARY := doxygen-mcp

build:
	go build ./cmd/doxygen-mcp

test:
	go test ./...

docker-build:
	docker build --target runtime -t $(IMAGE) .

docker-test:
	docker build --target test -t $(TEST_IMAGE) .
	docker run --rm $(TEST_IMAGE)

clean:
	rm -f $(BINARY)
