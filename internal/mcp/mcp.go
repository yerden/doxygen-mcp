package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/yerden/doxygen-mcp/internal/db"
)

func NewServer(database *db.DB) *server.MCPServer {
	s := server.NewMCPServer("doxygen-mcp", "0.1.0",
		server.WithToolCapabilities(false),
	)

	s.AddTool(mcp.NewTool("search",
		mcp.WithDescription("Full-text search over C symbols indexed from Doxygen XML. "+
			"Searches symbol names, signatures, and descriptions. "+
			"Symbols are snake_case; the index splits on underscores, so 'init' matches 'buf_init'. "+
			"Supports FTS5 syntax: prefix (init*), boolean (alloc AND free), phrase (\"open file\"). "+
			"Returns functions, macros, typedefs, structs, enums, and variables with file and line."),
		mcp.WithString("query", mcp.Required(), mcp.Description("FTS5 search query")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of results to return (default 20)")),
	), searchHandler(database))

	s.AddTool(mcp.NewTool("get_symbol",
		mcp.WithDescription("Look up a C symbol by exact name. "+
			"Returns kind, file location, return type, full signature, description, and parameter list."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Exact symbol name, e.g. buf_init")),
	), getSymbolHandler(database))

	s.AddTool(mcp.NewTool("list_files",
		mcp.WithDescription("List all source files in the index as project-relative paths. "+
			"Use these paths with symbols_in_file."),
	), listFilesHandler(database))

	s.AddTool(mcp.NewTool("symbols_in_file",
		mcp.WithDescription("List all symbols defined in a source file. "+
			"Use list_files to get valid paths. "+
			"Paths are relative to the project root (e.g. src/buf.c), not absolute. "+
			"Leading ./ is accepted."),
		mcp.WithString("file", mcp.Required(),
			mcp.Description("Project-relative path to the source file, e.g. src/buf.c")),
	), symbolsInFileHandler(database))

	return s
}

func searchHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := req.GetString("query", "")
		limit := req.GetInt("limit", 20)
		rows, err := database.Query("search_symbols", query, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search error: %v", err)), nil
		}
		defer rows.Close()
		var sb strings.Builder
		for rows.Next() {
			var id int64
			var name, kind, sig, desc, file string
			var line int
			if err := rows.Scan(&id, &name, &kind, &sig, &desc, &file, &line); err != nil {
				continue
			}
			fmt.Fprintf(&sb, "[%s] %s (%s:%d)\n", kind, name, file, line)
			if sig != "" {
				fmt.Fprintf(&sb, "  signature: %s\n", sig)
			}
			if desc != "" {
				fmt.Fprintf(&sb, "  description: %s\n", desc)
			}
		}
		if sb.Len() == 0 {
			return mcp.NewToolResultText("No results found."), nil
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}

func getSymbolHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := req.GetString("name", "")
		rows, err := database.Query("get_symbol_by_name", name)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("lookup error: %v", err)), nil
		}
		defer rows.Close()
		var sb strings.Builder
		for rows.Next() {
			var id int64
			var symName, kind, sig, desc, retType, file string
			var line int
			if err := rows.Scan(&id, &symName, &kind, &sig, &desc, &retType, &file, &line); err != nil {
				continue
			}
			fmt.Fprintf(&sb, "Name: %s\nKind: %s\nFile: %s:%d\n", symName, kind, file, line)
			if retType != "" {
				fmt.Fprintf(&sb, "Returns: %s\n", retType)
			}
			if sig != "" {
				fmt.Fprintf(&sb, "Signature: %s\n", sig)
			}
			if desc != "" {
				fmt.Fprintf(&sb, "Description: %s\n", desc)
			}
			prows, err := database.Query("get_params", id)
			if err == nil {
				defer prows.Close()
				first := true
				for prows.Next() {
					if first {
						fmt.Fprintf(&sb, "Parameters:\n")
						first = false
					}
					var pos int
					var pname, ptype, pdesc string
					if err := prows.Scan(&pos, &pname, &ptype, &pdesc); err != nil {
						continue
					}
					fmt.Fprintf(&sb, "  %d. %s %s", pos, ptype, pname)
					if pdesc != "" {
						fmt.Fprintf(&sb, " — %s", pdesc)
					}
					fmt.Fprintln(&sb)
				}
			}
			fmt.Fprintln(&sb)
		}
		if sb.Len() == 0 {
			return mcp.NewToolResultText("Symbol not found."), nil
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}

func listFilesHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rows, err := database.Query("list_files")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("error: %v", err)), nil
		}
		defer rows.Close()
		var sb strings.Builder
		for rows.Next() {
			var path string
			if err := rows.Scan(&path); err != nil {
				continue
			}
			fmt.Fprintln(&sb, path)
		}
		if sb.Len() == 0 {
			return mcp.NewToolResultText("No files indexed."), nil
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}

func symbolsInFileHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		file := filepath.Clean(req.GetString("file", ""))
		if filepath.IsAbs(file) {
			return mcp.NewToolResultError("file path must be relative, got absolute path: " + file), nil
		}
		rows, err := database.Query("symbols_in_file", file)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("error: %v", err)), nil
		}
		defer rows.Close()
		var sb strings.Builder
		for rows.Next() {
			var id int64
			var name, kind, sig string
			var line int
			if err := rows.Scan(&id, &name, &kind, &line, &sig); err != nil {
				continue
			}
			fmt.Fprintf(&sb, "[%s] %s (line %d)", kind, name, line)
			if sig != "" {
				fmt.Fprintf(&sb, " — %s", sig)
			}
			fmt.Fprintln(&sb)
		}
		if sb.Len() == 0 {
			return mcp.NewToolResultText("No symbols found."), nil
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}
