package db

import (
	"bufio"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed sql/schema.sql
var schemaSQL string

//go:embed sql/queries.sql
var queriesSQL string

type DB struct {
	*sql.DB
	queries map[string]string
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := conn.Exec(schemaSQL); err != nil {
		conn.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	q, err := loadQueries()
	if err != nil {
		conn.Close()
		return nil, err
	}
	return &DB{DB: conn, queries: q}, nil
}

func loadQueries() (map[string]string, error) {
	queries := make(map[string]string)
	var name string
	var buf strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(queriesSQL))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "-- name:") {
			if name != "" {
				queries[name] = strings.TrimSpace(buf.String())
			}
			name = strings.TrimSpace(strings.TrimPrefix(line, "-- name:"))
			buf.Reset()
		} else {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	if name != "" {
		queries[name] = strings.TrimSpace(buf.String())
	}
	return queries, scanner.Err()
}

func (d *DB) Query(name string, args ...any) (*sql.Rows, error) {
	q, ok := d.queries[name]
	if !ok {
		return nil, fmt.Errorf("unknown query: %s", name)
	}
	return d.DB.Query(q, args...)
}

func (d *DB) QueryRow(name string, args ...any) (*sql.Row, error) {
	q, ok := d.queries[name]
	if !ok {
		return nil, fmt.Errorf("unknown query: %s", name)
	}
	return d.DB.QueryRow(q, args...), nil
}

func (d *DB) Exec(name string, args ...any) (sql.Result, error) {
	q, ok := d.queries[name]
	if !ok {
		return nil, fmt.Errorf("unknown query: %s", name)
	}
	return d.DB.Exec(q, args...)
}

func (d *DB) ExecRaw(query string, args ...any) (sql.Result, error) {
	return d.DB.Exec(query, args...)
}
