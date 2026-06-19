package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/yerden/doxygen-mcp/internal/db"
	"github.com/yerden/doxygen-mcp/internal/indexer"
)

func main() {
	xmlDir := flag.String("xml", "", "Doxygen XML output directory")
	dbPath := flag.String("db", "", "SQLite database path")
	flag.Parse()
	if *xmlDir == "" || *dbPath == "" {
		fmt.Fprintln(os.Stderr, "usage: indexer --xml <dir> --db <path>")
		os.Exit(1)
	}
	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()
	if err := indexer.Run(*xmlDir, database); err != nil {
		log.Fatalf("index: %v", err)
	}
	log.Println("indexing complete")
}
