# gotreesitter documentation site

The documentation and browser playground for
[`github.com/odvcencio/gotreesitter`](https://github.com/odvcencio/gotreesitter), built with
[GoSX](https://github.com/odvcencio/gosx). Pages render on the server; the language catalog and
`/playground` use dedicated standard-Go GoSX engines, with release-keyed grammar blobs loaded only
by the playground.

Live site: [gotreesitter.m31labs.dev](https://gotreesitter.m31labs.dev/)

## Local development

Requirements:

- Go 1.26
- GoSX 0.31.4 (`go install m31labs.dev/gosx/cmd/gosx@v0.31.4`)
- TinyGo 0.41.1 for the production GoSX runtime

```sh
cp .env.example .env
gosx dev .
```

The interactive playground runtime is generated and intentionally untracked:

```sh
./scripts/build-playground-wasm.sh
```

That script builds `app/playground/client` with the standard Go `js/wasm` target and
`grammar_subset`, plus the smaller `app/langsearch/client` engine, then stages both modules and
deterministic gzip sidecars under `public/`. GoSX supplies the generated toolchain `wasm_exec.js`;
this repository ships no application-authored JavaScript or TypeScript.

## Production verification

Run the complete source, build, export, size, and route smoke gate:

```sh
./scripts/verify-production.sh
```

The gate derives the exact GoSX and gotreesitter versions from `go.mod`, rejects local module
replacements, validates every `.gsx` source, runs the Go tests, creates a clean `gosx build --prod`
artifact, runs `gosx export`, reports runtime size, boots `dist/run.sh`, and requests every
documentation route plus the playground APIs and browser assets. It also proves that the
release-versioned parser runtime is served from its gzip sidecar with immutable caching. The `gosx`
binary on `PATH` must exactly match the version in `go.mod`.

Browser performance budgets are optional because they require a local Chrome installation:

```sh
RUN_BROWSER_SMOKE=1 ./scripts/verify-production.sh
RUN_BROWSER_PERF=1 ./scripts/verify-production.sh
```

The smoke checks the standard-Go language filter, waits for the playground surface to enable the
editor, then covers the complete language picker, sample chips, live predicate queries, and the
syntax tree's roving keyboard focus. The performance mode includes that smoke before collecting
route budgets.

The public CI workflow installs the exact GoSX and TinyGo versions declared by this repository,
then runs module verification, tests, vet, GoSX source checks, the production build/export, and the
same route, API, asset, compression, cache-policy, and real-browser interaction gate.

## Deployment contract

Deploy the complete `dist/` directory and start `dist/run.sh`. The server needs the copied `app/`,
`content/`, and `public/` trees to resolve file routes and documentation at runtime.

```sh
PORT=8080 GOSX_ENV=production ./dist/run.sh
curl --fail http://localhost:8080/healthz
```

`gosx export` currently pre-renders the landing route only. The documentation collection and WASM
playground are dynamic routes, and auto-detection is a JSON API, so
the static export alone is not a complete deployment.

Require intermediaries to preserve and honor the origin's `Cache-Control` response. Grant immutable
caching only to exact content-hashed or release-versioned URLs; do not blanket-cache `/gosx/*`.
Hashed `/assets/*` resources, the content-versioned language-search runtime, and exact-version
playground resources may be cached immutably when the origin says so; unversioned API and runtime
URLs revalidate or use `no-store`.

The canonical deployment manifests and Harbor image promotion live in the private `m31labs.dev`
infrastructure repository. This source repository intentionally carries no registry credentials,
cluster secrets, or duplicate Kubernetes manifests; deployment automation consumes the complete
verified `dist/` artifact described above.

## Playground privacy boundary

Parsing and query execution happen in the browser. Automatic language detection sends a bounded
source sample to the server (request body capped at 64 KiB; the first 4 KiB is scored). Selecting a
language before typing avoids that request and keeps source fully local.

## License

This documentation site is available under the [MIT License](LICENSE).

## Performance claims

User-visible numbers must match gotreesitter's release-pinned
[`BENCH.md`](https://github.com/odvcencio/gotreesitter/blob/v0.36.0/BENCH.md). In particular,
withdrawn no-tree diagnostics must never be presented as materialized full-parse performance.
