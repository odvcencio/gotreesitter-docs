#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# The production gate must resolve exactly the checked-in module graph. A
# parent workspace or a writable module mode can otherwise make a local
# checkout pass with code that is not present in go.mod/go.sum.
export GOWORK=off
export GOFLAGS=-mod=readonly

fail() {
  echo "error: $*" >&2
  exit 1
}

if [[ -n "${GTS_LOCAL_DIR:-}" ]]; then
  fail "GTS_LOCAL_DIR is a development override and is forbidden by the production verifier"
fi

for command in go gosx tinygo curl gzip grep sed awk cmp wc mktemp; do
  if ! command -v "$command" >/dev/null 2>&1; then
    fail "required command not found: $command"
  fi
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
  [[ -n "$version" ]] || fail "cannot read $module version from go list -m -json"
  printf '%s\n' "$version"
}

gosx_version="$(module_version m31labs.dev/gosx)"
gts_version="$(module_version github.com/odvcencio/gotreesitter)"
[[ "$gosx_version" == v* ]] || fail "invalid GoSX module version from go.mod: $gosx_version"
[[ "$gts_version" == v* ]] || fail "invalid gotreesitter module version from go.mod: $gts_version"

gosx_cli_output="$(gosx version 2>&1)"
gosx_cli_version="${gosx_cli_output##* }"
if [[ "$gosx_cli_version" != "$gosx_version" ]]; then
  fail "gosx CLI $gosx_cli_version does not match go.mod $gosx_version; install m31labs.dev/gosx/cmd/gosx@$gosx_version"
fi
grep -Fq "GoSX ${gosx_version#v}" README.md || fail "README GoSX requirement does not match go.mod $gosx_version"
grep -Fq "m31labs.dev/gosx/cmd/gosx@$gosx_version" README.md || fail "README GoSX install command does not match go.mod $gosx_version"
grep -Fq "github.com/odvcencio/gotreesitter@$gts_version" app/page.gsx || \
  fail "landing-page install command does not match go.mod $gts_version"
bench_url="https://github.com/odvcencio/gotreesitter/blob/$gts_version/BENCH.md"
for source in README.md content/docs/performance.md content/docs/incremental-parsing.md; do
  grep -Fq "$bench_url" "$source" || fail "$source benchmark link does not match go.mod $gts_version"
done
if grep -REn \
  '1[.]54[[:space:]]*ms|649[[:space:]]*ns|2[.]43[[:space:]]*ns|41[,]?800' \
  README.md app content; then
  fail "withdrawn benchmark claim remains in public site source"
fi

rm -rf build dist

./scripts/build-playground-wasm.sh

while IFS= read -r source; do
  gosx check "$source"
done < <(find app -name '*.gsx' -type f | sort)

go mod verify
go test ./...
go vet ./...
gosx build --prod .
gosx export .

size_report="$(mktemp)"
gosx size --json dist | tee "$size_report"

test -x dist/run.sh
test -x dist/server/app
test -s dist/build.json
test -s dist/public/playground/runtime.wasm
test -s dist/public/playground/runtime.wasm.gz

port="${VERIFY_PORT:-18080}"
base="http://127.0.0.1:${port}"
server_log="$(mktemp)"
server_pid=""
temp_files=("$size_report" "$server_log")
temp_path=""

new_temp_file() {
  temp_path="$(mktemp)"
  temp_files+=("$temp_path")
}

cleanup() {
  if [[ -n "$server_pid" ]]; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -f "${temp_files[@]}"
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
  slug="$(basename "$source" .md)"
  routes+=("/docs/$slug")
done
for route in "${routes[@]}"; do
  curl --silent --show-error --fail --output /dev/null "$base$route"
done

curl --silent --fail "$base/" | grep -q '1.895'
curl --silent --fail "$base/docs/performance" | grep -q '33,000'

assert_revalidating_response() {
  local url="$1"
  local body_pattern="$2"
  local headers
  local body
  new_temp_file
  headers="$temp_path"
  new_temp_file
  body="$temp_path"
  curl --silent --show-error --fail --max-time 30 \
    --dump-header "$headers" --output "$body" "$base$url"
  grep -Fq "$body_pattern" "$body" || fail "unexpected response body for $url"
  if ! grep -Eiq '^Cache-Control:.*(no-cache|max-age=0.*must-revalidate)' "$headers"; then
    fail "unversioned response does not require revalidation: $url"
  fi
  if grep -Eiq '^Cache-Control:.*immutable' "$headers"; then
    fail "unversioned response is immutable: $url"
  fi
}

assert_immutable_response() {
  local url="$1"
  local body_pattern="$2"
  local headers
  local body
  new_temp_file
  headers="$temp_path"
  new_temp_file
  body="$temp_path"
  curl --silent --show-error --fail --max-time 30 \
    --dump-header "$headers" --output "$body" "$base$url"
  grep -Fq "$body_pattern" "$body" || fail "unexpected response body for $url"
  grep -Eiq '^Cache-Control:.*max-age=31536000.*immutable' "$headers" || \
    fail "exact-version response is not immutable: $url"
}

assert_revalidating_response "/playground/langs.json" "\"version\":\"$gts_version\""
assert_immutable_response "/playground/langs.json?v=$gts_version" "\"version\":\"$gts_version\""
assert_revalidating_response "/playground/lang/go.json" '"name":"go"'
assert_immutable_response "/playground/lang/go.json?v=$gts_version" '"name":"go"'

new_temp_file
languages_html="$temp_path"
curl --silent --show-error --fail --max-time 15 --output "$languages_html" "$base/docs/languages"
lang_search_url="$(grep -oE '/gosx/islands/LangSearch\.json\?v=[0-9a-f]{12}' "$languages_html" | head -n 1)"
[[ -n "$lang_search_url" ]] || fail "cannot find content-versioned LangSearch island URL"
assert_revalidating_response "/gosx/islands/LangSearch.json" 'LangSearch'
assert_immutable_response "$lang_search_url" 'LangSearch'

new_temp_file
detect_response="$temp_path"
curl --silent --show-error --fail --max-time 30 \
  -H 'Content-Type: application/json' \
  --data-binary '{"source":"package main\nfunc main(){}"}' \
  --output "$detect_response" \
  "$base/playground/detect"
grep -q '"ok":true' "$detect_response"
grep -q '"best":"go"' "$detect_response"

new_temp_file
playground_html="$temp_path"
curl --silent --show-error --fail --max-time 15 --output "$playground_html" "$base/playground"
runtime_url="$(sed -n 's/.*data-wasm-url="\([^"]*\)".*/\1/p' "$playground_html" | head -n 1)"
wasm_exec_url="$(sed -n 's/.*src="\([^"]*playground\/wasm_exec\.js[^"]*\)".*/\1/p' "$playground_html" | head -n 1)"
playground_js_url="$(sed -n 's/.*src="\([^"]*playground\/playground\.js[^"]*\)".*/\1/p' "$playground_html" | head -n 1)"
playground_css_url="$(sed -n 's/.*href="\([^"]*playground\/playground\.css[^"]*\)".*/\1/p' "$playground_html" | head -n 1)"

[[ "$runtime_url" == "/playground/runtime.wasm?v=$gts_version" ]] || fail "unexpected runtime URL: $runtime_url"
[[ "$wasm_exec_url" =~ ^/playground/wasm_exec\.js\?v=[0-9a-f]{12}$ ]] || fail "unexpected wasm_exec URL: $wasm_exec_url"
[[ "$playground_js_url" =~ ^/playground/playground\.js\?v=[0-9a-f]{12}$ ]] || fail "unexpected playground JS URL: $playground_js_url"
[[ "$playground_css_url" =~ ^/playground/playground\.css\?v=[0-9a-f]{12}$ ]] || fail "unexpected playground CSS URL: $playground_css_url"
grep -Eq 'id="pg-status"[^>]*role="status"[^>]*aria-live="polite"' "$playground_html" || fail "playground status is not an ARIA live status"
grep -Eq 'id="pg-qerr"[^>]*role="alert"' "$playground_html" || fail "playground query errors are not alerts"
grep -Eq 'id="pg-tree"[^>]*role="tree"[^>]*aria-label="Syntax tree"' "$playground_html" || fail "playground syntax tree has no accessible name"

assert_public_asset() {
  local url="$1"
  local content_type_pattern="$2"
  local headers
  new_temp_file
  headers="$temp_path"
  curl --silent --show-error --fail --max-time 15 --head --dump-header "$headers" --output /dev/null "$base$url"
  grep -Eiq "^Content-Type:.*$content_type_pattern" "$headers" || fail "unexpected Content-Type for $url"
  grep -Eiq '^Cache-Control:.*max-age=0.*must-revalidate' "$headers" || fail "unexpected Cache-Control for $url"
}
assert_public_asset "$wasm_exec_url" 'javascript'
assert_public_asset "$playground_js_url" 'javascript'
assert_public_asset "$playground_css_url" 'text/css'

new_temp_file
playground_js_body="$temp_path"
curl --silent --show-error --fail --max-time 15 --output "$playground_js_body" "$base$playground_js_url"
grep -Fq 'row.setAttribute("role", "treeitem")' "$playground_js_body" || fail "playground tree rows are not treeitems"
grep -Fq 'case "ArrowDown":' "$playground_js_body" || fail "playground tree has no ArrowDown handling"
grep -Fq 'case "ArrowUp":' "$playground_js_body" || fail "playground tree has no ArrowUp handling"
grep -Fq 'case "Home":' "$playground_js_body" || fail "playground tree has no Home handling"
grep -Fq 'case "End":' "$playground_js_body" || fail "playground tree has no End handling"
grep -Fq 'case "Enter":' "$playground_js_body" || fail "playground tree has no activation handling"

new_temp_file
playground_css_body="$temp_path"
curl --silent --show-error --fail --max-time 15 --output "$playground_css_body" "$base$playground_css_url"
grep -Fq '.pg-select:focus-visible' "$playground_css_body" || fail "playground select has no visible keyboard focus style"
grep -Fq '.pg-src:focus-visible' "$playground_css_body" || fail "playground source editor has no visible keyboard focus style"
grep -Fq '.pg-qsrc:focus-visible' "$playground_css_body" || fail "playground query editor has no visible keyboard focus style"
grep -Fq '.pg-tline:focus-visible' "$playground_css_body" || fail "playground tree rows have no visible keyboard focus style"

new_temp_file
runtime_headers="$temp_path"
new_temp_file
runtime_body="$temp_path"
curl --silent --show-error --fail --max-time 30 \
  -H 'Accept-Encoding: gzip' \
  --dump-header "$runtime_headers" \
  --output "$runtime_body" \
  "$base$runtime_url"
grep -Eiq '^Content-Type:[[:space:]]*application/wasm' "$runtime_headers" || fail "runtime MIME type is not application/wasm"
grep -Eiq '^Content-Encoding:[[:space:]]*gzip' "$runtime_headers" || fail "runtime did not use gzip sidecar"
grep -Eiq '^Vary:.*Accept-Encoding' "$runtime_headers" || fail "runtime response does not vary on Accept-Encoding"
grep -Eiq '^Cache-Control:.*max-age=31536000.*immutable' "$runtime_headers" || fail "versioned runtime is not immutable"
grep -Eiq '^ETag:' "$runtime_headers" || fail "runtime response has no ETag"
cmp -s "$runtime_body" dist/public/playground/runtime.wasm.gz || fail "runtime response differs from staged gzip sidecar"

compressed_bytes="$(wc -c <"$runtime_body" | tr -d '[:space:]')"
sidecar_bytes="$(wc -c <dist/public/playground/runtime.wasm.gz | tr -d '[:space:]')"
[[ "$compressed_bytes" == "$sidecar_bytes" ]] || fail "runtime transfer size $compressed_bytes differs from sidecar $sidecar_bytes"
(( compressed_bytes <= 4500000 )) || fail "compressed runtime exceeds 4.5 MB: $compressed_bytes bytes"

new_temp_file
runtime_head_headers="$temp_path"
curl --silent --show-error --fail --max-time 15 --head \
  -H 'Accept-Encoding: gzip' \
  --dump-header "$runtime_head_headers" \
  --output /dev/null \
  "$base$runtime_url"
grep -Eiq "^Content-Length:[[:space:]]*$sidecar_bytes" "$runtime_head_headers" || fail "HEAD runtime length does not match gzip sidecar"

new_temp_file
runtime_identity_headers="$temp_path"
curl --silent --show-error --fail --max-time 30 \
  -H 'Accept-Encoding: gzip;q=0' \
  --dump-header "$runtime_identity_headers" \
  --output /dev/null \
  "$base$runtime_url"
if grep -Eiq '^Content-Encoding:' "$runtime_identity_headers"; then
  fail "identity runtime response unexpectedly has Content-Encoding"
fi
raw_bytes="$(wc -c <dist/public/playground/runtime.wasm | tr -d '[:space:]')"
grep -Eiq "^Content-Length:[[:space:]]*$raw_bytes" "$runtime_identity_headers" || fail "identity runtime length does not match raw WASM"

new_temp_file
runtime_preferred_identity_headers="$temp_path"
curl --silent --show-error --fail --max-time 30 \
  -H 'Accept-Encoding: identity;q=0.9, gzip;q=0.1' \
  --dump-header "$runtime_preferred_identity_headers" \
  --output /dev/null \
  "$base$runtime_url"
if grep -Eiq '^Content-Encoding:' "$runtime_preferred_identity_headers"; then
  fail "lower-quality gzip was preferred over identity"
fi
grep -Eiq "^Content-Length:[[:space:]]*$raw_bytes" "$runtime_preferred_identity_headers" || fail "preferred identity length does not match raw WASM"

new_temp_file
runtime_not_acceptable_headers="$temp_path"
not_acceptable_status="$(curl --silent --show-error --max-time 15 \
  -H 'Accept-Encoding: *;q=0' \
  --dump-header "$runtime_not_acceptable_headers" \
  --output /dev/null \
  --write-out '%{http_code}' \
  "$base$runtime_url")"
[[ "$not_acceptable_status" == "406" ]] || fail "runtime unacceptable-encoding status is $not_acceptable_status, want 406"
grep -Eiq '^Vary:.*Accept-Encoding' "$runtime_not_acceptable_headers" || fail "runtime 406 response does not vary on Accept-Encoding"

new_temp_file
runtime_unversioned_headers="$temp_path"
curl --silent --show-error --fail --max-time 15 --head \
  -H 'Accept-Encoding: gzip' \
  --dump-header "$runtime_unversioned_headers" \
  --output /dev/null \
  "$base/playground/runtime.wasm"
grep -Eiq '^Cache-Control:.*max-age=0.*must-revalidate' "$runtime_unversioned_headers" || fail "unversioned runtime is cache-immutable"
if grep -Eiq '^Cache-Control:.*immutable' "$runtime_unversioned_headers"; then
  fail "unversioned runtime is cache-immutable"
fi

etag="$(awk 'BEGIN { IGNORECASE=1 } /^ETag:/ { sub(/^[^:]*:[[:space:]]*/, ""); sub(/\r$/, ""); print; exit }' "$runtime_headers")"
[[ -n "$etag" ]] || fail "cannot read runtime ETag"
conditional_status="$(curl --silent --show-error --max-time 15 \
  -H 'Accept-Encoding: gzip' \
  -H "If-None-Match: $etag" \
  --output /dev/null \
  --write-out '%{http_code}' \
  "$base$runtime_url")"
[[ "$conditional_status" == "304" ]] || fail "runtime conditional status is $conditional_status, want 304"

if [[ "${RUN_BROWSER_PERF:-0}" == "1" ]]; then
  for route in / /docs/getting-started /docs/performance; do
    gosx perf --coverage --budget perf-budget.json --budget-profile docs "$base$route"
  done
  gosx perf --coverage --budget perf-budget.json --budget-profile playground "$base/playground"
fi

echo "production verification passed"
echo "verified GoSX $gosx_version with gotreesitter $gts_version"
echo "compressed playground runtime: $compressed_bytes bytes"
