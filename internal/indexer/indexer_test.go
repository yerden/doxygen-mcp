package indexer

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/yerden/doxygen-mcp/internal/db"
)

func TestIndexer(t *testing.T) {
	if _, err := exec.LookPath("doxygen"); err != nil {
		t.Skip("doxygen not in PATH")
	}

	projectRoot, err := filepath.Abs("../../testdata/sample-c")
	if err != nil {
		t.Fatal(err)
	}
	doxyfile, err := filepath.Abs("../../Doxyfile")
	if err != nil {
		t.Fatal(err)
	}

	xmlBase := t.TempDir()
	cmd := exec.Command("doxygen", doxyfile)
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(),
		"DOXYGEN_OUTPUT_DIR="+xmlBase,
		"DOXYGEN_INPUT="+projectRoot,
		"DOXYGEN_STRIP_FROM_PATH="+projectRoot,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("doxygen failed: %v\n%s", err, out)
	}

	xmlDir := filepath.Join(xmlBase, "xml")

	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if err := Run(xmlDir, database); err != nil {
		t.Fatalf("Run: %v", err)
	}

	rows, err := database.Query("list_files")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var files []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
	}
	if len(files) == 0 {
		t.Error("expected indexed files, got none")
	}

	// Search for known symbol
	srows, err := database.Query("search_symbols", "add", 10)
	if err != nil {
		t.Fatal(err)
	}
	defer srows.Close()
	found := false
	for srows.Next() {
		var id int64
		var name, kind, sig, desc, file string
		var line int
		if err := srows.Scan(&id, &name, &kind, &sig, &desc, &file, &line); err != nil {
			t.Fatal(err)
		}
		if name == "add" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find 'add' function")
	}
}

func TestStripXMLTags(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello", "hello"},
		{"<ref>foo</ref>", "foo"},
		{"int <ref>mytype</ref> *", "int mytype *"},
	}
	for _, c := range cases {
		got := stripXMLTags(c.in)
		if got != c.want {
			t.Errorf("stripXMLTags(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
