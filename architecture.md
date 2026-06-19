# doxygen-mcp Architecture

## Overview

An MCP server that indexes Doxygen-generated XML into a SQLite FTS5 database and
exposes search/lookup tools to MCP clients. Written in Go.

---

## Directory Layout

```
doxygen-mcp/
├── cmd/
│   ├── indexer/          # CLI: parse Doxygen XML → populate SQLite
│   └── server/           # MCP server binary
├── internal/
│   ├── indexer/          # XML parsing logic (encoding/xml)
│   ├── db/               # Database layer: open, migrate, query
│   └── mcp/              # MCP tool handlers
├── sql/
│   ├── schema.sql        # DDL: tables + FTS5 virtual table
│   └── queries.sql       # Named queries (parsed at startup)
├── testdata/
│   └── sample-c/         # Minimal C project used in tests
├── Doxyfile              # Doxygen config for C projects
├── Dockerfile
├── Makefile
└── go.mod
```

---

## Components

### 1. Doxygen Configuration (`Doxyfile`)

Target: C projects. Key settings:

| Setting | Value | Reason |
|---|---|---|
| `GENERATE_XML` | `YES` | Only XML output needed |
| `GENERATE_HTML` | `NO` | Disable unused outputs |
| `EXTRACT_ALL` | `YES` | Include undocumented symbols |
| `EXTRACT_PRIVATE` | `YES` | Private functions |
| `EXTRACT_STATIC` | `YES` | Static (file-local) functions |
| `EXTRACT_ANON_NSPACES` | `YES` | Anonymous structs |
| `RECURSIVE` | `YES` | Walk subdirectories |
| `XML_PROGRAMLISTING` | `NO` | Skip source dumps (save space) |
| `STRIP_FROM_PATH` | `$(PROJECT_ROOT)` | Strip absolute prefix so XML contains paths relative to project root |

`STRIP_FROM_PATH` is the only setting that needs to vary per project. It should
be set to the project root (the directory passed to `INPUT`). Doxygen then writes
`src/foo.c` into `<location file="..."/>` instead of `/home/user/project/src/foo.c`.

The indexer stores these relative paths verbatim — no runtime flag needed, and
the same SQLite database is valid regardless of where the source tree is checked
out on the host.

---

### 2. SQLite Schema (`sql/schema.sql`)

```sql
CREATE TABLE IF NOT EXISTS files (
    id   INTEGER PRIMARY KEY,
    path TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS symbols (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL,   -- function | struct | enum | typedef | define | variable
    file_id     INTEGER REFERENCES files(id),
    line        INTEGER,
    signature   TEXT,            -- full function signature for functions
    description TEXT,            -- brief + detailed from Doxygen
    return_type TEXT             -- functions only
);

CREATE TABLE IF NOT EXISTS params (
    id          INTEGER PRIMARY KEY,
    symbol_id   INTEGER NOT NULL REFERENCES symbols(id),
    position    INTEGER NOT NULL,
    name        TEXT,
    type        TEXT,
    description TEXT
);

CREATE VIRTUAL TABLE IF NOT EXISTS fts USING fts5(
    name,
    signature,
    description,
    content='symbols',
    content_rowid='id'
);

-- Keep FTS in sync
CREATE TRIGGER IF NOT EXISTS symbols_ai AFTER INSERT ON symbols BEGIN
    INSERT INTO fts(rowid, name, signature, description)
    VALUES (new.id, new.name, new.signature, new.description);
END;
```

---

### 3. Named Queries (`sql/queries.sql`)

A single file, queries delimited by `-- name: <key>` comments:

```sql
-- name: search_symbols
SELECT s.id, s.name, s.kind, s.signature, s.description,
       f.path AS file, s.line
FROM fts
JOIN symbols s ON s.id = fts.rowid
JOIN files   f ON f.id = s.file_id
WHERE fts MATCH ?
ORDER BY rank
LIMIT ?;

-- name: get_symbol_by_name
SELECT s.*, f.path AS file
FROM symbols s
JOIN files f ON f.id = s.file_id
WHERE s.name = ?;

-- name: get_params
SELECT position, name, type, description
FROM params
WHERE symbol_id = ?
ORDER BY position;

-- name: list_files
SELECT path FROM files ORDER BY path;

-- name: symbols_in_file
SELECT id, name, kind, line, signature
FROM symbols s
JOIN files f ON f.id = s.file_id
WHERE f.path = ?
ORDER BY line;
```

Parsed at startup into `map[string]string` by splitting on `-- name:`.

---

### 4. Go Packages

#### `internal/db`

- `Open(path string) (*DB, error)` — open SQLite, run `schema.sql` migration
- `LoadQueries() (map[string]string, error)` — parse `queries.sql`
- `DB` wraps `*sql.DB` with a `Query(name string, args ...any)` helper that
  looks up the SQL by name and executes it

SQL files embedded via `//go:embed`:

```go
//go:embed ../../sql/schema.sql
var schemaSQL string

//go:embed ../../sql/queries.sql
var queriesSQL string
```

#### `internal/indexer`

- Walk Doxygen XML output directory
- Parse `*.xml` with `encoding/xml` into intermediate structs matching Doxygen's
  compound schema
- Insert files, symbols, params into SQLite via `internal/db`
- File paths stored as-is from `<location file="..."/>` — already relative thanks
  to `STRIP_FROM_PATH` in the Doxyfile
- Wipes and re-indexes on every run (no incremental/mtime logic)

Key Doxygen XML elements to handle:
- `<compounddef kind="file">` → source files
- `<memberdef kind="function">` → functions
- `<memberdef kind="define">` → macros
- `<memberdef kind="typedef">` → typedefs
- `<memberdef kind="variable">` → global/static vars
- `<compounddef kind="struct|union|enum">` → types

#### `internal/mcp`

Implements MCP tools. Uses `mark3labs/mcp-go`.

**Tools exposed:**

| Tool | Description | Parameters |
|---|---|---|
| `search` | FTS5 full-text search | `query: string`, `limit?: int` |
| `get_symbol` | Exact name lookup + params | `name: string` |
| `list_files` | All indexed source files | — |
| `symbols_in_file` | Symbols in a specific file | `file: string` |

#### `cmd/indexer`

```
indexer --xml <doxygen-xml-dir> --db <sqlite-path>
```

#### `cmd/server`

```
server --db <sqlite-path>
```

MCP transport: stdio only (for use with Claude Desktop / claude CLI).

---

### 5. Dockerfile

Multi-stage build. The container expects pre-generated Doxygen XML as a bind
mount — it does not run `doxygen` itself.

```
Stage 1 (build): golang:1.24-alpine
  - Build indexer and server binaries

Stage 2 (runtime): alpine
  - Copy binaries
  - ENTRYPOINT: run indexer on mounted XML dir, then exec server (stdio)

Stage 3 (test): built on top of stage 1
  - Run go test ./...
```

Typical invocation (e.g. from claude CLI MCP config):

```
docker run -i --rm \
  -v /path/to/xml:/xml:ro \
  -v /path/to/index.db:/data/index.db \
  doxygen-mcp
```

Run tests:

```
docker build --target test -t doxygen-mcp-test .
docker run --rm doxygen-mcp-test
```

---

### 6. Testing

All tests run inside Docker (`make test` → `docker build --target test` + `docker run --rm`).

- `internal/db`: schema migration, query loading, basic CRUD
- `internal/indexer`: parse sample Doxygen XML from `testdata/`
- `internal/mcp`: tool handler unit tests with in-memory SQLite
- Integration test: full pipeline — run Doxygen on `testdata/sample-c/`,
  index, query via MCP tool

---

## Data Flow

```
C source files
     │
     ▼ doxygen (Doxyfile)
Doxygen XML (xml/*.xml)
     │
     ▼ cmd/indexer
SQLite (FTS5 indexed)
     │
     ▼ cmd/server (MCP stdio transport)
MCP Client (claude CLI, Claude Desktop, etc.)
```

---

## Decisions

| # | Decision |
|---|---|
| MCP library | `mark3labs/mcp-go` |
| Re-indexing | Wipe and reindex on every start |
| Docker / Doxygen | Container expects pre-generated XML; does not run `doxygen` |
| Transport | stdio only |
