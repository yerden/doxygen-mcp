package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/yerden/doxygen-mcp/internal/db"
)

func setupDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	if _, err := d.ExecRaw("INSERT INTO files(path) VALUES (?)", "src/foo.c"); err != nil {
		t.Fatal(err)
	}
	var fileID int64
	if err := d.DB.QueryRow("SELECT id FROM files WHERE path='src/foo.c'").Scan(&fileID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ExecRaw(
		"INSERT INTO symbols(name,kind,file_id,line,signature,description,return_type) VALUES (?,?,?,?,?,?,?)",
		"compute", "function", fileID, 42, "int compute(int x)", "computes result", "int",
	); err != nil {
		t.Fatal(err)
	}
	var symID int64
	if err := d.DB.QueryRow("SELECT id FROM symbols WHERE name='compute'").Scan(&symID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ExecRaw(
		"INSERT INTO params(symbol_id,position,name,type,description) VALUES (?,?,?,?,?)",
		symID, 0, "x", "int", "input value",
	); err != nil {
		t.Fatal(err)
	}
	return d
}

func makeReq(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func TestSearchHandler(t *testing.T) {
	d := setupDB(t)
	h := searchHandler(d)
	res, err := h(context.Background(), makeReq(map[string]any{"query": "compute"}))
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %v", res.Content)
	}
	text := extractText(res)
	if text == "" || text == "No results found." {
		t.Error("expected search results")
	}
}

func TestGetSymbolHandler(t *testing.T) {
	d := setupDB(t)
	h := getSymbolHandler(d)
	res, err := h(context.Background(), makeReq(map[string]any{"name": "compute"}))
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(res)
	if text == "Symbol not found." {
		t.Error("expected to find 'compute'")
	}
}

func TestListFilesHandler(t *testing.T) {
	d := setupDB(t)
	h := listFilesHandler(d)
	res, err := h(context.Background(), makeReq(nil))
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(res)
	if text == "No files indexed." {
		t.Error("expected files")
	}
}

func TestSymbolsInFileHandler(t *testing.T) {
	d := setupDB(t)
	h := symbolsInFileHandler(d)
	res, err := h(context.Background(), makeReq(map[string]any{"file": "src/foo.c"}))
	if err != nil {
		t.Fatal(err)
	}
	text := extractText(res)
	if text == "No symbols found." {
		t.Error("expected symbols")
	}
}

func extractText(res *mcp.CallToolResult) string {
	for _, c := range res.Content {
		if t, ok := c.(mcp.TextContent); ok {
			return t.Text
		}
	}
	return ""
}
