-- name: search_symbols
SELECT s.id, s.name, s.kind, s.signature, s.description,
       f.path AS file, s.line
FROM fts
JOIN symbols s ON s.id = fts.rowid
JOIN files   f ON f.id = s.file_id
WHERE fts MATCH ?
ORDER BY rank
LIMIT ?;

-- name: get_symbol_by_name
SELECT s.id, s.name, s.kind, s.signature, s.description, s.return_type, f.path AS file, s.line
FROM symbols s
JOIN files f ON f.id = s.file_id
WHERE s.name = ?;

-- name: get_params
SELECT position, name, type, description
FROM params
WHERE symbol_id = ?
ORDER BY position;

-- name: list_files
SELECT path FROM files ORDER BY path;

-- name: symbols_in_file
SELECT s.id, s.name, s.kind, s.line, s.signature
FROM symbols s
JOIN files f ON f.id = s.file_id
WHERE f.path = ?
ORDER BY s.line;
