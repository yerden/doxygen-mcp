#!/bin/sh
set -e

# Resolve --db from the forwarded serve args; otherwise use ${DB_PATH}. The
# resolved value is passed to both `index` and `serve` so they always agree.
db="${DB_PATH}"
set -- "$@" '<<END>>'
while :; do
    case "$1" in
        '<<END>>') shift; break ;;
        --db) db="$2"; shift 2 ;;
        --db=*) db="${1#--db=}"; shift ;;
        *) set -- "$@" "$1"; shift ;;
    esac
done

doxygen-mcp index --xml "${XML_DIR}" --db "$db"
exec doxygen-mcp serve --db "$db" "$@"
