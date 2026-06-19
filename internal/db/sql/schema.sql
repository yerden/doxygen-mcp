CREATE TABLE IF NOT EXISTS files (
    id   INTEGER PRIMARY KEY,
    path TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS symbols (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL,
    file_id     INTEGER REFERENCES files(id),
    line        INTEGER,
    signature   TEXT,
    description TEXT,
    return_type TEXT
);

CREATE TABLE IF NOT EXISTS params (
    id          INTEGER PRIMARY KEY,
    symbol_id   INTEGER NOT NULL REFERENCES symbols(id),
    position    INTEGER NOT NULL,
    name        TEXT,
    type        TEXT,
    description TEXT
);

CREATE VIRTUAL TABLE IF NOT EXISTS fts USING fts5(
    name,
    signature,
    description,
    content='symbols',
    content_rowid='id',
    tokenize="unicode61 separators '_'"
);

CREATE TRIGGER IF NOT EXISTS symbols_ai AFTER INSERT ON symbols BEGIN
    INSERT INTO fts(rowid, name, signature, description)
    VALUES (new.id, new.name, new.signature, new.description);
END;
