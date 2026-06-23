package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"
	dbpkg "github.com/yerden/doxygen-mcp/internal/db"
	"github.com/yerden/doxygen-mcp/internal/indexer"
	mcppkg "github.com/yerden/doxygen-mcp/internal/mcp"
)

func main() {
	xmlDir := flag.String("xml", "", "Doxygen XML output directory (required)")
	dbPath := flag.String("db", "", "SQLite database path; omit for in-memory")
	httpAddr := flag.String("http", "", "HTTP listen address (e.g. :9123); omit for stdio mode")
	flag.Parse()

	if *xmlDir == "" {
		fmt.Fprintln(os.Stderr, "usage: doxygen-mcp --xml <dir> [--db <path>] [--http <addr>]")
		os.Exit(1)
	}

	path := *dbPath
	if path == "" {
		path = ":memory:"
	}
	database, err := dbpkg.Open(path)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	if err := indexer.Run(*xmlDir, database); err != nil {
		log.Fatalf("index: %v", err)
	}

	s := mcppkg.NewServer(database)

	if *httpAddr != "" {
		log.Printf("serving MCP over HTTP on %s", *httpAddr)
		hs := server.NewStreamableHTTPServer(s)
		if err := hs.Start(*httpAddr); err != nil {
			log.Fatalf("http server: %v", err)
		}
		return
	}

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("server: %v", err)
	}
}
