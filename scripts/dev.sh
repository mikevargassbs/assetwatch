#!/usr/bin/env bash
# Runs the backend and, if the embedded frontend is stale or missing,
# rebuilds it first — so `scripts/dev.sh` is the one command that gets you
# both frontend and backend from `./cmd/api`.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
web="$root/web"
dist="$web/dist"

newest_mtime() {
    find "$@" -type f -printf '%T@\n' 2>/dev/null | sort -n | tail -1
}

dist_built=$(newest_mtime "$dist/assets" 2>/dev/null || true)
source_changed=$(newest_mtime "$web/src" "$web/index.html" "$web/package.json" "$web/vite.config.ts" 2>/dev/null || true)

needs_build=false
if [ -z "$dist_built" ]; then
    needs_build=true
elif [ -n "$source_changed" ] && awk -v a="$source_changed" -v b="$dist_built" 'BEGIN{exit !(a>b)}'; then
    needs_build=true
fi

if [ "$needs_build" = true ]; then
    echo "Frontend build is missing or stale — running npm run build..."
    if [ ! -d "$web/node_modules" ]; then
        npm --prefix "$web" install
    fi
    npm --prefix "$web" run build
else
    echo "Frontend build is up to date, skipping npm build."
fi

go run ./cmd/api
