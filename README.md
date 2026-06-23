# doxygen-mcp

An MCP server that indexes [Doxygen](https://www.doxygen.nl/)-generated XML into a SQLite FTS5 database and exposes search and lookup tools to MCP clients. Designed for C projects with snake_case naming conventions.

## Overview

```
C source files
     │
     ▼  doxygen
Doxygen XML  (xml/*.xml)
     │
     ▼  doxygen-mcp index --xml <dir> --db <path>
SQLite FTS5 database
     │
     ▼  doxygen-mcp serve --db <path> [--http <addr>]
MCP client (Claude Desktop, claude CLI, opencode, …)
```

The indexer parses Doxygen XML output and stores symbols (functions, macros, typedefs, structs, enums, variables) in a full-text-searchable SQLite database. The server exposes four MCP tools over stdio or HTTP.

## MCP Tools

| Tool | Description |
|---|---|
| `search` | FTS5 full-text search over symbol names, signatures, and descriptions. Supports prefix (`init*`), boolean (`alloc AND free`), and phrase (`"open file"`) syntax. Splits on underscores: `init` matches `buf_init`. |
| `get_symbol` | Exact name lookup. Returns kind, file location, return type, full signature, description, and parameters. |
| `list_files` | Lists all indexed source files as project-relative paths. |
| `symbols_in_file` | Lists all symbols defined in a source file. |

## Requirements

- Go 1.26+
- Doxygen (to generate XML input; not required to run the server)
- Docker (optional)

## Quick start

### 1. Generate Doxygen XML

Point Doxygen at your C project with `GENERATE_XML=YES`. The included `Doxyfile` is environment-variable driven:

```sh
export DOXYGEN_INPUT=/path/to/your/c/project
export DOXYGEN_OUTPUT_DIR=/tmp/doxygen-out
export DOXYGEN_STRIP_FROM_PATH=/path/to/your/c/project
doxygen Doxyfile
# XML appears in /tmp/doxygen-out/xml/
```

`DOXYGEN_STRIP_FROM_PATH` is important: it makes the XML contain project-relative paths (`src/foo.c`) rather than absolute host paths, so the database is valid regardless of where the source tree is checked out.

### 2. Build

```sh
make build
# produces ./doxygen-mcp
```

### 3. Index

```sh
./doxygen-mcp index --xml /tmp/doxygen-out/xml --db index.db
```

### 4. Run the MCP server

**HTTP transport** (recommended for Claude Desktop and most clients):

```sh
./doxygen-mcp serve --db index.db --http :9123
```

**Stdio transport** (for Claude Desktop subprocess mode):

```sh
./doxygen-mcp serve --db index.db
```

## Docker

The Docker image re-indexes on every start, then forwards all `docker run` arguments to `doxygen-mcp serve`. The default `CMD` is `--http :9123`.

```sh
# Build
docker build -t doxygen-mcp .

# Run (mount pre-generated Doxygen XML and a persistent data volume)
docker run --rm \
  -v /path/to/doxygen/xml:/xml:ro \
  -v /path/to/data:/data \
  -p 9123:9123 \
  doxygen-mcp
```

Override the default args to change transport / port / db path:

```sh
docker run --rm -p 8080:8080 doxygen-mcp --http :8080            # custom HTTP port
docker run --rm -i doxygen-mcp --db /data/index.db               # stdio mode
```

The entrypoint reads `--db` from the forwarded args (falling back to `$DB_PATH`) and uses the same value for both indexing and serving, so the two never disagree.

Environment variables:

| Variable | Default | Description |
|---|---|---|
| `XML_DIR` | `/xml` | Path to Doxygen XML directory inside the container |
| `DB_PATH` | `/data/index.db` | SQLite database path inside the container (used when `--db` isn't passed) |

The container does **not** run Doxygen. Mount pre-generated XML at `/xml`.

## Connecting MCP clients

**Claude CLI:**

```sh
claude mcp add --transport http doxygen http://localhost:9123/mcp
```

**Claude Desktop** (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "doxygen": {
      "type": "http",
      "url": "http://localhost:9123/mcp"
    }
  }
}
```

**Stdio (Claude Desktop subprocess):**

```json
{
  "mcpServers": {
    "doxygen": {
      "command": "/path/to/doxygen-mcp",
      "args": ["serve", "--db", "/path/to/index.db"]
    }
  }
}
```

## Development

```sh
make test           # go test ./...
make docker-test    # build test stage and run tests inside Docker (includes Doxygen)
make docker-build   # build the runtime Docker image
```

## Project structure

```
doxygen-mcp/
├── cmd/
│   └── doxygen-mcp/main.go   # Single binary: `index` and `serve` subcommands
├── internal/
│   ├── db/                   # SQLite open, schema migration, named query loader
│   ├── indexer/              # XML walking and DB insertion
│   └── mcp/                  # MCP tool definitions and handlers
├── testdata/
│   └── sample-c/src/         # Minimal C project used in tests
├── Doxyfile                  # Doxygen config for C projects (env-var driven)
└── Dockerfile                # Multi-stage: build / test / runtime
```

## Design notes

- **No CGo**: uses `modernc.org/sqlite`, a pure-Go SQLite port; binaries are fully static (`CGO_ENABLED=0`).
- **Single transaction for indexing**: avoids per-row auto-commit, which makes SQLite slow on large projects.
- **Snake_case tokenization**: FTS5 is configured with `tokenize="unicode61 separators '_'"` so that searching for `init` matches `buf_init`, `module_init`, etc.
- **SQL embedded in binary**: schema and queries live in `internal/db/sql/` and are baked in at compile time via `//go:embed`.
- **Re-index on every container start**: no incremental logic; simple and correct.

## License

MIT
