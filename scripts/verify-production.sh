#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

export GOWORK=off
export GOFLAGS=-mod=readonly

fail() {
  echo "error: $*" >&2
  exit 1
}

for command in go gosx tinygo curl grep sed awk mktemp find; do
  command -v "$command" >/dev/null 2>&1 || fail "required command not found: $command"
done

go_toolchain="$(awk '$1 == "toolchain" { print $2; exit }' go.mod)"
[[ -n "$go_toolchain" ]] || fail "go.mod has no toolchain directive"
[[ "$(go env GOVERSION)" == "$go_toolchain" ]] || fail "Go $(go env GOVERSION) does not match go.mod $go_toolchain"

tinygo_version="$(tinygo version | awk '{ print $3; exit }')"
[[ "$tinygo_version" == "0.41.1" ]] || fail "TinyGo $tinygo_version does not match required 0.41.1"

module_version() {
  local module="$1"
  local metadata
  local version
  metadata="$(go list -m -json "$module")"
  if grep -Eq '"Replace"[[:space:]]*:[[:space:]]*\{' <<<"$metadata"; then
    fail "$module is replaced in the resolved module graph"
  fi
  version="$(awk -F '"' '/^[[:space:]]*"Version"[[:space:]]*:/ { print $4; exit }' <<<"$metadata")"
  [[ -n "$version" ]] || fail "cannot read $module version"
  printf '%s\n' "$version"
}

gosx_version="$(module_version m31labs.dev/gosx)"
gts_version="$(module_version github.com/odvcencio/gotreesitter)"
gosx_cli_version="$(gosx version 2>&1)"
[[ "${gosx_cli_version##* }" == "$gosx_version" ]] || fail "gosx CLI does not match go.mod $gosx_version"
grep -Fq "GoSX ${gosx_version#v}" README.md || fail "README GoSX requirement does not match go.mod"
grep -Fq "m31labs.dev/gosx/cmd/gosx@$gosx_version" README.md || fail "README GoSX install command does not match go.mod"

# Application behavior must be authored in Go/GoSX. Framework-generated
# runtime assets in dist are allowed; repository-authored JS is not.
if find . -path './.git' -prune -o -path './dist' -prune -o -path './build' -prune \
  -o -type f \( -name '*.js' -o -name '*.jsx' -o -name '*.ts' -o -name '*.tsx' \) -print -quit | grep -q .; then
  fail "application-authored JavaScript remains in the repository"
fi
if grep -R -n --include='*.gsx' '<script' app; then
  fail "a .gsx component contains a script escape hatch"
fi

rm -rf build dist
go run ./cmd/build-playground-wasm
while IFS= read -r source; do
  gosx check "$source"
done < <(find app -name '*.gsx' -type f | sort)

go mod verify
go test ./...
go vet ./...
gosx build --prod .
gosx export .
gosx size --json dist

test -x dist/run.sh
test -x dist/server/app
test -s dist/build.json

port="${VERIFY_PORT:-18080}"
base="http://127.0.0.1:${port}"
server_log="$(mktemp)"
page_body="$(mktemp)"
server_pid=""

cleanup() {
  if [[ -n "$server_pid" ]]; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -f "$server_log" "$page_body"
}
trap cleanup EXIT

PORT="$port" GOSX_ENV=production ./dist/run.sh >"$server_log" 2>&1 &
server_pid=$!
for _ in $(seq 1 60); do
  if curl --silent --fail "$base/healthz" >/dev/null; then
    break
  fi
  if ! kill -0 "$server_pid" 2>/dev/null; then
    cat "$server_log" >&2
    exit 1
  fi
  sleep 0.25
done
curl --silent --fail "$base/healthz" | grep -q '"ok":true'

routes=("/" "/playground")
for source in content/docs/*.md; do
  routes+=("/docs/$(basename "$source" .md)")
done
for route in "${routes[@]}"; do
  curl --silent --show-error --fail --output /dev/null "$base$route"
done

curl --silent --fail "$base/" | grep -q '5.48×' || fail "landing headline metric (5.48× geomean) missing from /"
curl --silent --fail "$base/docs/performance" | grep -q '5.48× C' || fail "performance geomean (5.48× C) missing from /docs/performance"

languages_html="$(curl --silent --show-error --fail "$base/docs/languages")"
lang_search_url="$(grep -oE '/gosx/islands/LangSearch\.json\?v=[0-9a-f]{12}' <<<"$languages_html" | head -n 1)"
[[ -n "$lang_search_url" ]] || fail "cannot find content-versioned LangSearch island URL"
curl --silent --show-error --fail "$base$lang_search_url" | grep -q 'LangSearch'

# The server delivers a GoSX surface and immutable engine asset. It has no
# mutation endpoint, form submission, or route that can receive editor text.
curl --silent --show-error --fail --output "$page_body" "$base/playground"
grep -q 'data-gosx-engine="GotreesitterPlayground"' "$page_body" || fail "playground GoSX engine mount is missing"
grep -q 'data-gosx-main="true"' "$page_body" || fail "managed-navigation main boundary is missing"
grep -q 'data-gosx-navigation="true"' "$page_body" || fail "GoSX navigation runtime is missing"
if grep -Eq '<form|action="/playground|src="[^"]*playground\.js' "$page_body"; then
  fail "playground still exposes a server submission or bespoke JS path"
fi
wasm_url="$(grep -oE '/playground/runtime\.wasm\?v=[0-9a-f]{12}' "$page_body" | head -n 1)"
[[ -n "$wasm_url" ]] || fail "content-versioned playground WASM URL is missing"
curl --silent --show-error --fail --head "$base$wasm_url" | grep -Fqi 'content-type: application/wasm' || fail "playground engine is not served as application/wasm"

PLAYGROUND_BASE_URL="$base" go run ./cmd/verify-playground-browser

if [[ "${RUN_BROWSER_PERF:-0}" == "1" ]]; then
  for route in / /docs/getting-started /docs/performance /playground; do
    gosx perf --coverage --budget perf-budget.json --budget-profile docs "$base$route"
  done
fi

echo "production verification passed"
echo "verified GoSX $gosx_version with gotreesitter $gts_version"
