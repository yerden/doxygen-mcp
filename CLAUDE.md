# doxygen-mcp

An MCP server that indexes Doxygen-generated XML into a SQLite FTS5 database and exposes search/lookup tools to MCP clients. Written in Go. Targets C projects with snake_case naming conventions.

## Directory layout

```
doxygen-mcp/
├── cmd/
│   └── doxygen-mcp/main.go   # Single binary with `index` and `serve` subcommands
├── internal/
│   ├── db/
│   │   ├── db.go             # Open, schema migration, named query loader
│   │   └── sql/
│   │       ├── schema.sql    # DDL embedded into binary at compile time
│   │       └── queries.sql   # Named queries embedded into binary at compile time
│   ├── indexer/
│   │   ├── indexer.go        # XML walking, DB insertion (single transaction)
│   │   └── xml.go            # encoding/xml structs for Doxygen compound schema
│   └── mcp/
│       └── mcp.go            # MCP tool definitions and handlers
├── testdata/
│   └── sample-c/src/         # Minimal C project (math.h + math.c) used in tests
├── Doxyfile                  # Doxygen config for C projects (env-var driven)
├── Dockerfile                # Multi-stage: build / test / runtime
├── docker-entrypoint.sh      # Runs indexer then starts HTTP server on :9123
├── Makefile
└── go.mod                    # module github.com/yerden/doxygen-mcp
```

## Key dependencies

| Package | Purpose |
|---|---|
| `modernc.org/sqlite` | Pure-Go SQLite driver (no CGo) |
| `github.com/mark3labs/mcp-go` | MCP server library (tools, stdio, streamable HTTP) |

## Data flow

```
C source files
     │
     ▼  doxygen (Doxyfile)
Doxygen XML  (xml/*.xml)
     │
     ▼  doxygen-mcp index --xml <dir> --db <path>
SQLite FTS5 database
     │
     ▼  doxygen-mcp serve --db <path> [--http <addr>]
MCP client (Claude Desktop, claude CLI, opencode, …)
```

## internal/db

`Open(path)` opens SQLite, runs `schema.sql` (idempotent `CREATE IF NOT EXISTS`), then parses `queries.sql` into a `map[string]string`.

Both SQL files are embedded via `//go:embed sql/*.sql` — they live under `internal/db/sql/` (not at repo root) because Go embed cannot traverse `..`.

`loadQueries()` is a line tokenizer: lines starting with `-- name: <key>` delimit entries; everything else accumulates into the current query body. Call sites reference queries by snake_case name: `db.Query("search_symbols", q, limit)`.

`DB` exposes:
- `Query(name, args...)` — returns `*sql.Rows`
- `QueryRow(name, args...)` — returns `*sql.Row`
- `Exec(name, args...)` — returns `sql.Result`
- `ExecRaw(sql, args...)` — bypasses the named map (used for DDL and wipe)

## internal/indexer

`Run(xmlDir, db)` opens a single transaction, wipes existing data, walks `index.xml`, parses each compound XML file, and commits.

**Wipe order matters**: FTS5 external-content tables must be cleared with `INSERT INTO fts(fts) VALUES('delete-all')` before deleting rows from `params`, `symbols`, `files`. A plain `DELETE FROM fts` does not work correctly for content tables.

**Everything in one transaction**: without this, auto-commit per INSERT makes indexing extremely slow on large projects.

File paths are stored via `filepath.Clean()` to normalize `./`-prefixed paths from Doxygen.

Doxygen compound kinds handled:
- `file` → members become symbols (functions, defines, typedefs, variables)
- `struct`, `union`, `enum` → become symbols; their members become params

## internal/mcp

Four tools exposed via `mark3labs/mcp-go`:

| Tool | Description |
|---|---|
| `search` | FTS5 full-text search; snake_case tokenized; supports FTS5 syntax (`*`, `AND`, `OR`, phrase) |
| `get_symbol` | Exact name lookup; returns kind, file, line, return type, signature, description, params |
| `list_files` | Lists all indexed source files as project-relative paths |
| `symbols_in_file` | Lists all symbols in a file; rejects absolute paths with a clear error |

`symbols_in_file` normalizes the `file` parameter with `filepath.Clean` and rejects absolute paths via `filepath.IsAbs` with an explicit error (instead of "No symbols found").

## SQLite schema

Three regular tables (`files`, `symbols`, `params`) plus one FTS5 virtual table:

```sql
CREATE VIRTUAL TABLE fts USING fts5(
    name, signature, description,
    content='symbols',
    content_rowid='id',
    tokenize="unicode61 separators '_'"
);
```

`tokenize="unicode61 separators '_'"` splits on underscores so that `buf_init` is indexed as tokens `buf` and `init`. This means searching for `init` matches `buf_init`, `module_init`, etc.

The FTS table is an external-content table (`content='symbols'`): it does not store text itself, only the index. An AFTER INSERT trigger on `symbols` keeps it in sync during indexing.

**Quoting**: the tokenize option requires double quotes in `CREATE VIRTUAL TABLE`. Single quotes cause a parse error in SQLite's FTS5 directive parser.

## cmd/doxygen-mcp

Single binary with two subcommands. Dispatched on `os.Args[1]`; each subcommand uses its own `flag.FlagSet`.

```
doxygen-mcp index --xml <doxygen-xml-dir> --db <sqlite-path>
doxygen-mcp serve --db <sqlite-path> [--http <addr>]
```

`serve` transport modes:
- Without `--http`: stdio transport. Detects non-pipe stdin (e.g. Docker without `-i`, or a terminal) and exits with a clear error.
- With `--http`: streamable HTTP transport (MCP 2025-03-26 spec) via `server.NewStreamableHTTPServer`. Default endpoint path is `/mcp`.

The two transports are mutually exclusive per process.

## Doxyfile

Environment-variable driven — three vars must be set before invoking doxygen:

| Variable | Purpose |
|---|---|
| `DOXYGEN_INPUT` | Root directory of C source to document |
| `DOXYGEN_OUTPUT_DIR` | Where doxygen writes output (XML appears in `$DOXYGEN_OUTPUT_DIR/xml/`) |
| `DOXYGEN_STRIP_FROM_PATH` | Prefix stripped from absolute paths in `<location file="..."/>` |

`STRIP_FROM_PATH` is critical: it makes the XML contain project-relative paths (`src/foo.c`) rather than absolute host paths. The indexer stores these verbatim, so the same database is valid regardless of where the source tree is checked out.

Key settings: `EXTRACT_ALL=YES`, `EXTRACT_PRIVATE=YES`, `EXTRACT_STATIC=YES`, `GENERATE_XML=YES`, `GENERATE_HTML=NO`.

## Dockerfile

Three stages:

| Stage | Base | Purpose |
|---|---|---|
| `build` | `golang:1.26-alpine` | Compiles the `doxygen-mcp` binary |
| `test` | `build` + doxygen | Runs `go test -v ./...` |
| `runtime` | `alpine:3.21` | Ships binary + entrypoint; `runtime` is last so `docker build .` produces the runnable image |

The container does **not** run doxygen. It expects pre-generated XML mounted at `/xml`.

Environment variables with defaults:
- `XML_DIR=/xml`
- `DB_PATH=/data/index.db`

Port `9123` is exposed. `/xml` and `/data` are declared as volumes; without a `-v` bind-mount, Docker creates an anonymous volume that is destroyed on `docker rm`.

## docker-entrypoint.sh

```sh
# parse --db from "$@" (falls back to $DB_PATH)
doxygen-mcp index --xml "${XML_DIR}" --db "$db"
exec doxygen-mcp serve --db "$db" "$@"
```

Forwards all arguments to `serve`. The default `CMD` in the Dockerfile is `["--http", ":9123"]`, so `docker run doxygen-mcp` starts HTTP on :9123 unmodified. Users can override:

```sh
docker run doxygen-mcp --http :8080            # different HTTP port
docker run -i doxygen-mcp --db /data/index.db  # stdio mode
```

The entrypoint extracts `--db` from the forwarded args (or falls back to `$DB_PATH`) and passes that same value to both `index` and `serve` — so the database the indexer writes is always the database the server reads.

Re-indexes on every container start (full wipe + rebuild, no incremental logic), then execs the MCP server.

## Running

**Build and run tests locally:**
```sh
make test                   # go test ./...
make docker-test            # build test stage, run in Docker (includes doxygen)
```

**Run the container:**
```sh
docker run --rm \
  -v /path/to/doxygen/xml:/xml:ro \
  -v /path/to/index.db:/data/index.db \
  -p 9123:9123 \
  doxygen-mcp
```

**Configure Claude CLI:**
```sh
claude mcp add --transport http doxygen http://localhost:9123/mcp
```

**Configure Claude Desktop** (`claude_desktop_config.json`):
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

## Design decisions

- **No CGo**: `modernc.org/sqlite` is a pure-Go SQLite port; the binary is fully static (`CGO_ENABLED=0`).
- **Single transaction for indexing**: avoids per-row auto-commit which makes SQLite very slow.
- **SQL files embedded in binary**: `//go:embed` bakes them in at compile time; schema and queries are editable files but need no runtime deployment.
- **Named queries only**: no inline SQL strings at call sites; all queries referenced by snake_case key.
- **Re-index on every start**: no mtime or incremental logic; simplicity over efficiency.
- **HTTP transport on :9123 by default**: the Dockerfile `CMD` provides `--http :9123`; users can override on the `docker run` line.
- **Stdio transport also supported**: pass `--http` to use HTTP, omit it for stdio (Claude Desktop subprocess mode).
