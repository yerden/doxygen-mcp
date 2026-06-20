#!/bin/sh
set -e
indexer --xml "${XML_DIR}" --db "${DB_PATH}"
exec server --db "${DB_PATH}" --http :9123
