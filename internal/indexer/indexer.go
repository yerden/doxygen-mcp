package indexer

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yerden/doxygen-mcp/internal/db"
)

func Run(xmlDir string, database *db.DB) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := wipe(tx); err != nil {
		return fmt.Errorf("wipe: %w", err)
	}

	indexFile := filepath.Join(xmlDir, "index.xml")
	data, err := os.ReadFile(indexFile)
	if err != nil {
		return fmt.Errorf("read index.xml: %w", err)
	}
	var idx DoxygenIndex
	if err := xml.Unmarshal(data, &idx); err != nil {
		return fmt.Errorf("parse index.xml: %w", err)
	}

	fileCache := make(map[string]int64)
	for _, entry := range idx.Compounds {
		xmlPath := filepath.Join(xmlDir, entry.RefID+".xml")
		if err := indexCompound(tx, xmlPath, fileCache); err != nil {
			return fmt.Errorf("index %s: %w", entry.RefID, err)
		}
	}

	return tx.Commit()
}

func wipe(tx *sql.Tx) error {
	// FTS5 external-content table requires its own delete command
	if _, err := tx.Exec("INSERT INTO fts(fts) VALUES('delete-all')"); err != nil {
		return err
	}
	for _, table := range []string{"params", "symbols", "files"} {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return err
		}
	}
	return nil
}

func indexCompound(tx *sql.Tx, xmlPath string, fileCache map[string]int64) error {
	data, err := os.ReadFile(xmlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var df DoxygenFile
	if err := xml.Unmarshal(data, &df); err != nil {
		return fmt.Errorf("parse %s: %w", xmlPath, err)
	}
	c := &df.Compound
	switch c.Kind {
	case "file":
		return indexFile(tx, c, fileCache)
	case "struct", "union", "enum":
		return indexType(tx, c, fileCache)
	}
	return nil
}

func getOrCreateFile(tx *sql.Tx, path string, cache map[string]int64) (int64, error) {
	path = filepath.Clean(path)
	if id, ok := cache[path]; ok {
		return id, nil
	}
	res, err := tx.Exec("INSERT OR IGNORE INTO files(path) VALUES (?)", path)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		if err := tx.QueryRow("SELECT id FROM files WHERE path = ?", path).Scan(&id); err != nil {
			return 0, err
		}
	}
	cache[path] = id
	return id, nil
}

func descText(d Description) string {
	var parts []string
	for _, p := range d.Para {
		t := stripXMLTags(p.Text)
		if t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}

func stripXMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func stripLinkedText(s string) string {
	return stripXMLTags(s)
}

func indexFile(tx *sql.Tx, c *CompoundDef, fileCache map[string]int64) error {
	for _, m := range c.Members {
		filePath := m.Location.File
		if filePath == "" {
			filePath = c.Location.File
		}
		if filePath == "" {
			continue
		}
		fileID, err := getOrCreateFile(tx, filePath, fileCache)
		if err != nil {
			return err
		}
		if err := insertMember(tx, &m, fileID); err != nil {
			return err
		}
	}
	return nil
}

func indexType(tx *sql.Tx, c *CompoundDef, fileCache map[string]int64) error {
	filePath := c.Location.File
	if filePath == "" {
		return nil
	}
	fileID, err := getOrCreateFile(tx, filePath, fileCache)
	if err != nil {
		return err
	}
	desc := strings.TrimSpace(descText(c.BriefDesc) + " " + descText(c.DetailedDesc))
	res, err := tx.Exec(
		"INSERT INTO symbols(name, kind, file_id, line, description) VALUES (?,?,?,?,?)",
		c.Name, c.Kind, fileID, c.Location.Line, desc,
	)
	if err != nil {
		return err
	}
	symID, _ := res.LastInsertId()
	for i, m := range c.Members {
		if m.Kind == "enumvalue" {
			continue
		}
		if _, err := tx.Exec(
			"INSERT INTO params(symbol_id, position, name, type, description) VALUES (?,?,?,?,?)",
			symID, i, m.Name, stripLinkedText(m.Type.Text), descText(m.BriefDesc),
		); err != nil {
			return err
		}
	}
	return nil
}

func insertMember(tx *sql.Tx, m *MemberDef, fileID int64) error {
	kind := m.Kind
	switch kind {
	case "function", "define", "typedef", "variable":
	default:
		return nil
	}
	desc := strings.TrimSpace(descText(m.BriefDesc) + " " + descText(m.DetailDesc))
	sig := m.Definition + m.ArgsString
	res, err := tx.Exec(
		"INSERT INTO symbols(name, kind, file_id, line, signature, description, return_type) VALUES (?,?,?,?,?,?,?)",
		m.Name, kind, fileID, m.Location.Line, sig, desc, stripLinkedText(m.Type.Text),
	)
	if err != nil {
		return err
	}
	if kind != "function" {
		return nil
	}
	symID, _ := res.LastInsertId()
	for i, p := range m.Params {
		typ := stripLinkedText(p.Type.Text)
		if typ == "void" && p.DeclName == "" {
			continue
		}
		if _, err := tx.Exec(
			"INSERT INTO params(symbol_id, position, name, type, description) VALUES (?,?,?,?,?)",
			symID, i, p.DeclName, typ, descText(p.BriefDesc),
		); err != nil {
			return err
		}
	}
	return nil
}
