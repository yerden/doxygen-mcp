FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/doxygen-mcp ./cmd/doxygen-mcp

FROM build AS test
RUN apk add --no-cache doxygen
CMD ["go", "test", "-v", "./..."]

FROM alpine:3.21 AS runtime
RUN apk add --no-cache sqlite
COPY --from=build /out/doxygen-mcp /usr/local/bin/doxygen-mcp
ENV DB_PATH=/data/index.db
ENV XML_DIR=/xml
EXPOSE 9123
VOLUME ["/xml", "/data"]
COPY docker-entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["--http", ":9123"]
