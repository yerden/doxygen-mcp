.PHONY: build test docker-build docker-test clean

IMAGE := doxygen-mcp
TEST_IMAGE := doxygen-mcp-test

build:
	go build ./cmd/indexer ./cmd/server

test:
	go test ./...

docker-build:
	docker build --target runtime -t $(IMAGE) .

docker-test:
	docker build --target test -t $(TEST_IMAGE) .
	docker run --rm $(TEST_IMAGE)

clean:
	rm -f indexer server
