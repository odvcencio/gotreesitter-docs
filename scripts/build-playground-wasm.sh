#!/usr/bin/env bash
# Build the site's two standard-Go GoSX engines: the parser playground and the
# lightweight language-catalog search.
#
# app/playground/client is an ordinary GOOS=js GOARCH=wasm package. It embeds
# the real parser/query/highlight runtime but no grammar blobs; languages load
# on demand from /playground/lang/{name}.json. The module registers the
# GoTreesitterPlayground factory through GoSX's engine/wasm package, so this
# script contains no client-side JavaScript generation or registration shim.
#
# The staged runtime and deterministic gzip sidecar are gitignored. GoSX owns
# the standard toolchain wasm_exec.js asset used by every Go-WASM engine.
#
#   ./scripts/build-playground-wasm.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PLAYGROUND_OUT="$ROOT/public/playground"
SEARCH_OUT="$ROOT/public/lang-search"

echo "building GoTreesitterPlayground from app/playground/client"
mkdir -p "$PLAYGROUND_OUT" "$SEARCH_OUT"
(
  cd "$ROOT" &&
    GOOS=js GOARCH=wasm \
    go build -tags grammar_subset -trimpath -ldflags='-s -w' \
      -o "$PLAYGROUND_OUT/runtime.wasm" ./app/playground/client
)

echo "building GoTreesitterLangSearch from app/langsearch/client"
(
  cd "$ROOT" &&
    GOOS=js GOARCH=wasm \
    go build -trimpath -ldflags='-s -w' \
      -o "$SEARCH_OUT/runtime.wasm" ./app/langsearch/client
)

# .gz sidecar so a deploy target (CDN / ingress) can serve the compressed
# body without compressing 14MB on the fly. Omit the source name and mtime so
# identical release runtimes produce byte-identical sidecars.
gzip -9 -n -kf "$PLAYGROUND_OUT/runtime.wasm"
gzip -9 -n -kf "$SEARCH_OUT/runtime.wasm"

echo "staged standard-Go engines:"
ls -la "$PLAYGROUND_OUT" "$SEARCH_OUT" | grep -E 'runtime\.wasm' || true
