# doxygen-mcp

An MCP server that indexes [Doxygen](https://www.doxygen.nl/)-generated XML into a SQLite FTS5 database and exposes search and lookup tools to MCP clients. Designed for C projects with snake_case naming conventions.

## Overview

```
C source files
     │
     ▼  doxygen
Doxygen XML  (xml/*.xml)
     │
     ▼  doxygen-mcp --xml <dir> [--db <path>] [--http <addr>]
SQLite FTS5 database (in-memory by default; --db persists to disk)
     │
     ▼
MCP client (Claude Desktop, claude CLI, opencode, …)
```

The binary indexes Doxygen XML and serves MCP in a single process. By default the index lives in RAM and is rebuilt on every start. Pass `--db <path>` to persist it to a SQLite file (still re-indexed on each start). Symbols (functions, macros, typedefs, structs, enums, variables) are stored full-text-searchable. Four MCP tools are exposed over stdio or HTTP.

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

`DOXYGEN_STRIP_FROM_PATH` is important: it makes the XML contain project-relative paths (`src/foo.c`) rather than absolute host paths.

### 2. Build

```sh
make build
# produces ./doxygen-mcp
```

### 3. Run

**HTTP transport** (recommended for Claude Desktop and most clients):

```sh
./doxygen-mcp --xml /tmp/doxygen-out/xml --http :9123
```

**Stdio transport** (for Claude Desktop subprocess mode):

```sh
./doxygen-mcp --xml /tmp/doxygen-out/xml
```

**Persist the index to disk** (optional; still rebuilt on each start):

```sh
./doxygen-mcp --xml /tmp/doxygen-out/xml --db index.db --http :9123
```

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
      "args": ["--xml", "/path/to/doxygen-out/xml"]
    }
  }
}
```

## Development

```sh
make test    # go test ./...
make build   # produces ./doxygen-mcp
```

## Project structure

```
doxygen-mcp/
├── cmd/
│   └── doxygen-mcp/main.go   # Single binary, single entry point
├── internal/
│   ├── db/                   # SQLite open, schema migration, named query loader
│   ├── indexer/              # XML walking and DB insertion
│   └── mcp/                  # MCP tool definitions and handlers
├── testdata/
│   └── sample-c/src/         # Minimal C project used in tests
└── Doxyfile                  # Doxygen config for C projects (env-var driven)
```

## Design notes

- **No CGo**: uses `modernc.org/sqlite`, a pure-Go SQLite port; binaries are fully static (`CGO_ENABLED=0`).
- **In-memory by default**: the index lives in RAM unless `--db` is given. The whole symbol table for a typical C project is small enough that this is fine.
- **Re-index on every start**: no incremental logic. Even with `--db`, the file is wiped and rebuilt — persistence is a convenience for external inspection (e.g. with the `sqlite3` CLI), not for skipping work.
- **Single transaction for indexing**: avoids per-row auto-commit, which makes SQLite slow on large projects.
- **Snake_case tokenization**: FTS5 is configured with `tokenize="unicode61 separators '_'"` so that searching for `init` matches `buf_init`, `module_init`, etc.
- **SQL embedded in binary**: schema and queries live in `internal/db/sql/` and are baked in at compile time via `//go:embed`.

## License

MIT
