package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"
	dbpkg "github.com/yerden/doxygen-mcp/internal/db"
	mcppkg "github.com/yerden/doxygen-mcp/internal/mcp"
)

func main() {
	dbPath := flag.String("db", "", "SQLite database path")
	httpAddr := flag.String("http", "", "HTTP listen address (e.g. :8080); omit for stdio mode")
	flag.Parse()
	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "usage: server --db <path> [--http <addr>]")
		os.Exit(1)
	}
	database, err := dbpkg.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	s := mcppkg.NewServer(database)

	if *httpAddr != "" {
		log.Printf("serving MCP over HTTP on %s", *httpAddr)
		hs := server.NewStreamableHTTPServer(s)
		if err := hs.Start(*httpAddr); err != nil {
			log.Fatalf("http server: %v", err)
		}
		return
	}

	fi, err := os.Stdin.Stat()
	if err != nil || (fi.Mode()&os.ModeCharDevice) != 0 {
		log.Fatal("stdin is not connected — run with: docker run -i ... or add --http <addr>")
	}
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("server: %v", err)
	}
}
