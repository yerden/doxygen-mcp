package db

import (
	"testing"
)

func TestOpen(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()
}

func TestLoadQueries(t *testing.T) {
	q, err := loadQueries()
	if err != nil {
		t.Fatalf("loadQueries: %v", err)
	}
	for _, name := range []string{"search_symbols", "get_symbol_by_name", "get_params", "list_files", "symbols_in_file"} {
		if _, ok := q[name]; !ok {
			t.Errorf("missing query: %s", name)
		}
	}
}

func TestCRUD(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	if _, err := d.ExecRaw("INSERT INTO files(path) VALUES (?)", "src/foo.c"); err != nil {
		t.Fatalf("insert file: %v", err)
	}
	var fileID int64
	if err := d.DB.QueryRow("SELECT id FROM files WHERE path='src/foo.c'").Scan(&fileID); err != nil {
		t.Fatalf("select file: %v", err)
	}
	if _, err := d.ExecRaw(
		"INSERT INTO symbols(name,kind,file_id,line,signature,description,return_type) VALUES (?,?,?,?,?,?,?)",
		"myfunc", "function", fileID, 10, "int myfunc(void)", "does something", "int",
	); err != nil {
		t.Fatalf("insert symbol: %v", err)
	}

	rows, err := d.Query("search_symbols", "myfunc", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var id int64
		var name, kind, sig, desc, file string
		var line int
		if err := rows.Scan(&id, &name, &kind, &sig, &desc, &file, &line); err != nil {
			t.Fatal(err)
		}
		if name == "myfunc" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find myfunc via FTS search")
	}
}
