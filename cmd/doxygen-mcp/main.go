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
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "index":
		runIndex(os.Args[2:])
	case "serve":
		runServe(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: doxygen-mcp <command> [flags]

Commands:
  index   Parse Doxygen XML and populate the SQLite database
  serve   Run the MCP server (stdio or HTTP)

Run 'doxygen-mcp <command> -h' for command flags.`)
}

func runIndex(args []string) {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	xmlDir := fs.String("xml", "", "Doxygen XML output directory")
	dbPath := fs.String("db", "", "SQLite database path")
	_ = fs.Parse(args)
	if *xmlDir == "" || *dbPath == "" {
		fmt.Fprintln(os.Stderr, "usage: doxygen-mcp index --xml <dir> --db <path>")
		os.Exit(1)
	}
	database, err := dbpkg.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()
	if err := indexer.Run(*xmlDir, database); err != nil {
		log.Fatalf("index: %v", err)
	}
	log.Println("indexing complete")
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	dbPath := fs.String("db", "", "SQLite database path")
	httpAddr := fs.String("http", "", "HTTP listen address (e.g. :8080); omit for stdio mode")
	_ = fs.Parse(args)
	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "usage: doxygen-mcp serve --db <path> [--http <addr>]")
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
