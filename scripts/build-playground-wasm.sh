#!/usr/bin/env bash
# Build the gotreesitter WASM runtime for the /playground page and stage it
# into public/playground/.
#
# The runtime is the real production engine (parser + query engine + blob
# loader) compiled for js/wasm with -tags grammar_subset, which embeds NO
# grammar blobs — the playground fetches per-language blobs from
# /playground/lang/{name}.json and registers them client-side through
# gotreesitter.loadBlob. Measured output: ~14.1MB raw / ~3.7MB gzipped.
#
# The staged artifacts (runtime.wasm, runtime.wasm.gz, wasm_exec.js) are
# gitignored — this script is the checked-in artifact. Run it after cloning
# and after every gotreesitter version bump:
#
#   ./scripts/build-playground-wasm.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/public/playground"

# Resolve the gotreesitter module dir from the docs repo's own go.mod so the
# staged runtime always matches the version the server links against.
#
# GTS_LOCAL_DIR overrides that resolution with a local engine checkout — for
# staging bridge features that are not in a released gotreesitter yet. A
# runtime built this way is a local dev artifact: DO NOT deploy it until the
# engine release containing the bridge ships and go.mod is bumped to it.
if [ -n "${GTS_LOCAL_DIR:-}" ]; then
  MODDIR="$GTS_LOCAL_DIR"
  echo "NOTE: building from GTS_LOCAL_DIR override (dev-only artifact, not deployable)"
else
  MODDIR="$(cd "$ROOT" && GOWORK=off go list -m -f '{{.Dir}}' github.com/odvcencio/gotreesitter)"
fi
if [ -z "$MODDIR" ] || [ ! -d "$MODDIR" ]; then
  echo "error: cannot resolve github.com/odvcencio/gotreesitter module dir" >&2
  exit 1
fi

echo "building runtime.wasm from $MODDIR"
mkdir -p "$OUT"
(
  cd "$MODDIR" &&
    GOWORK=off GOOS=js GOARCH=wasm \
      go build -tags grammar_subset -trimpath -ldflags='-s -w' \
      -o "$OUT/runtime.wasm" ./wasm/runtime
)
install -m 644 "$MODDIR/wasm/wasm_exec.js" "$OUT/wasm_exec.js"

# .gz sidecar so a deploy target (CDN / ingress) can serve the compressed
# body without compressing 14MB on the fly.
gzip -9 -kf "$OUT/runtime.wasm"

echo "staged into $OUT:"
ls -la "$OUT" | grep -E 'runtime\.wasm|wasm_exec\.js' || true
