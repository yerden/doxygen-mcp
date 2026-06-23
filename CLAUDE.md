# doxygen-mcp

An MCP server that indexes Doxygen-generated XML into a SQLite FTS5 database and exposes search/lookup tools to MCP clients. Written in Go. Targets C projects with snake_case naming conventions.

## Directory layout

```
doxygen-mcp/
├── cmd/
│   └── doxygen-mcp/main.go   # Single binary, single entry point (no subcommands)
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
     ▼  doxygen-mcp --xml <dir> [--db <path>] [--http <addr>]
SQLite FTS5 database (in-memory by default; --db persists to disk)
     │
     ▼
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

Single binary, single entry point. One `flag.Parse`, no subcommands.

```
doxygen-mcp --xml <doxygen-xml-dir> [--db <sqlite-path>] [--http <addr>]
```

- `--xml` is required.
- `--db` omitted → in-memory SQLite (`:memory:`). Given → file-backed at that path. Either way the indexer wipes and rebuilds on every start; persistence is purely a convenience for external inspection.
- Transport modes (mutually exclusive):
  - Without `--http`: stdio transport. Always serves on stdin/stdout — no terminal check, so any client that spawns the binary as a subprocess (Claude Desktop, opencode, etc.) can connect.
  - With `--http`: streamable HTTP transport (MCP 2025-03-26 spec) via `server.NewStreamableHTTPServer`. Default endpoint path is `/mcp`.

Flow on startup: open DB → run indexer (wipe + rebuild in a single transaction) → start MCP server.

## Doxyfile

Environment-variable driven — three vars must be set before invoking doxygen:

| Variable | Purpose |
|---|---|
| `DOXYGEN_INPUT` | Root directory of C source to document |
| `DOXYGEN_OUTPUT_DIR` | Where doxygen writes output (XML appears in `$DOXYGEN_OUTPUT_DIR/xml/`) |
| `DOXYGEN_STRIP_FROM_PATH` | Prefix stripped from absolute paths in `<location file="..."/>` |

`STRIP_FROM_PATH` is critical: it makes the XML contain project-relative paths (`src/foo.c`) rather than absolute host paths. The indexer stores these verbatim, so the same database is valid regardless of where the source tree is checked out.

Key settings: `EXTRACT_ALL=YES`, `EXTRACT_PRIVATE=YES`, `EXTRACT_STATIC=YES`, `GENERATE_XML=YES`, `GENERATE_HTML=NO`.

## Running

**Build and test locally:**
```sh
make build   # produces ./doxygen-mcp
make test    # go test ./...
```

**Run the server:**
```sh
./doxygen-mcp --xml /path/to/doxygen/xml --http :9123
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
- **In-memory by default**: `--db` is opt-in. The whole symbol table fits comfortably in RAM for typical C projects.
- **Re-index on every start**: no mtime or incremental logic. Even with `--db`, the file is wiped and rebuilt — persistence is for external inspection (e.g. `sqlite3 index.db`), not for skipping work.
- **Single transaction for indexing**: avoids per-row auto-commit which makes SQLite very slow.
- **SQL files embedded in binary**: `//go:embed` bakes them in at compile time; schema and queries are editable files but need no runtime deployment.
- **Named queries only**: no inline SQL strings at call sites; all queries referenced by snake_case key.
- **Transport selection**: pass `--http <addr>` for streamable HTTP; omit it for stdio (Claude Desktop subprocess mode).
