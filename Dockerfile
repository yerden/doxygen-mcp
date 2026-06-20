FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/indexer ./cmd/indexer && \
    CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM build AS test
RUN apk add --no-cache doxygen
CMD ["go", "test", "-v", "./..."]

FROM alpine:3.21 AS runtime
RUN apk add --no-cache sqlite
COPY --from=build /out/indexer /usr/local/bin/indexer
COPY --from=build /out/server  /usr/local/bin/server
ENV DB_PATH=/data/index.db
ENV XML_DIR=/xml
EXPOSE 9123
VOLUME ["/xml", "/data"]
COPY docker-entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
