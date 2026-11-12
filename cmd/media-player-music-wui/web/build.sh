#!/bin/sh
# Builds the web UI's page into ../internal/frontend, which the Go server
# embeds: index.html as it is, src/main.ts bundled into app.js. Needs tsc
# and esbuild; goframework's TypeScript client is taken from the workspace
# ($GOFRAMEWORK, by default ~/go/src/goframework.io) until it is published.
# src/musicapi.ts is generated from the Go API: go generate ./musicapi.
set -eu
cd "$(dirname "$0")"
GOFRAMEWORK="${GOFRAMEWORK:-$HOME/go/src/goframework.io}"
client="$GOFRAMEWORK/app-client-typescript/src/index.ts"
[ -f "$client" ] || { echo "build.sh: no $client (set GOFRAMEWORK)" >&2; exit 1; }
tsc -p tsconfig.json
esbuild src/main.ts --bundle --format=esm --target=es2022 \
	--alias:app-client-typescript="$client" \
	--sourcemap --minify \
	--outfile=../internal/frontend/app.js
cp index.html ../internal/frontend/index.html
