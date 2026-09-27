# gotreesitter documentation site

The documentation and browser playground for
[`github.com/odvcencio/gotreesitter`](https://github.com/odvcencio/gotreesitter), built with
[GoSX](https://github.com/odvcencio/gosx). Pages render on the server, the language catalog uses a
GoSX-authored island, and `/playground` runs gotreesitter in a standard-Go WebAssembly engine owned
by a GoSX surface. The engine indexes all 206 languages and fetches content-hashed grammar blobs
on demand. Every internal route uses managed navigation. The repository contains no
application-authored JavaScript.

Live site: [gotreesitter.m31labs.dev](https://gotreesitter.m31labs.dev/)

The site documents gotreesitter **v0.55.1**, released on 2026-09-26.
The site links its displayed parser version to the pinned module build.
The [v0.54.0 audit](docs/release-v0.54.0-audit.md) records the earlier content review.
The release catalog includes the v0.55.0 and v0.55.1 fixes and their measurement limits. The
server-rendered [/changelog](https://gotreesitter.m31labs.dev/changelog) route
turns the source changelog into a searchable release archive with tag, code,
pull request, issue, commit, and exact source-line evidence.

## GopherCon slide deck

The repository also contains a self-contained `gosx-slides` presentation in
[`slides/`](slides/README.md). It carries the 21-slide talk, presenter notes,
the supplied paper-and-ink visual system, and a live GLR benchmark island.

## Local development

Requirements:

- Go 1.26
- GoSX 0.57.1 (`go install m31labs.dev/gosx/cmd/gosx@v0.57.1`)
- TinyGo 0.41.1 for the production GoSX runtime
- Chrome or Chromium for the browser privacy/navigation gate

```sh
cp .env.example .env
go run ./cmd/build-playground-wasm
gosx dev .
```

## Production verification

Run the complete source, build, export, size, and route smoke gate:

```sh
./scripts/verify-production.sh
```

The gate derives the exact GoSX and gotreesitter versions from `go.mod`. It rejects local engine
overrides. It builds the playground, all 206 lazy grammar assets, and the authoring engine from Go.
It validates every `.gsx` source and runs the Go tests. It creates a clean `gosx build --prod`
artifact, runs `gosx export`, reports runtime size, boots `dist/run.sh`, and requests every route.
It checks every internal link and validates the authoring asset surface. Its Chrome check proves
that playground source never enters a request. It also proves that internal navigation does not
request a new document. The `gosx` binary on `PATH` must exactly match the version in `go.mod`.

Run the browser performance budgets when Chrome is available:

```sh
RUN_BROWSER_PERF=1 ./scripts/verify-production.sh
```

The playground has a 320 KiB decoded-JavaScript limit for the pinned GoSX engine, Go shim,
navigation, and analytics. The v0.54.0 audit measured about 297 KiB; the prior 160 KiB limit
was below the required framework payload. The 5,200 KiB network limit is unchanged.
The build writes a Brotli variant of the parser WASM, and the server negotiates that variant
without changing the decoded bytes or the versioned URL.

The public CI workflow installs the exact GoSX and TinyGo versions declared by this repository,
then runs module verification, tests, vet, GoSX source checks, the production build/export, and the
same route, browser-privacy, navigation, asset, cache-policy, and performance-budget gate.

## Deployment contract

Deploy the complete `dist/` directory and start `dist/run.sh`. The server needs the copied `app/`,
`content/`, and `public/` trees to resolve file routes and documentation at runtime.

```sh
PORT=8080 GOSX_ENV=production ./dist/run.sh
curl --fail http://localhost:8080/healthz
```

`gosx export` currently pre-renders the landing route only. The documentation collection and
playground shell are dynamic routes, so the static export alone is not a complete deployment.

Require intermediaries to preserve and honor the origin's `Cache-Control` response. Grant immutable
caching only to exact content-hashed or release-versioned URLs; do not blanket-cache `/gosx/*`.
Unversioned island and grammar-index URLs revalidate, while hashed `/assets/*` resources, the
content-versioned playground engine, and content-hashed grammar blobs may be cached immutably.

The canonical deployment manifests and Harbor image promotion live in the private `m31labs.dev`
infrastructure repository. This source repository intentionally carries no registry credentials,
cluster secrets, or duplicate Kubernetes manifests; deployment automation consumes the complete
verified `dist/` artifact described above.

## Playground privacy boundary

Parsing and query execution happen inside the browser in `public/playground/runtime.wasm`, a
standard-Go module registered through GoSX's engine runtime. The server sends the initial shell,
framework runtime, parser binary, and immutable grammar assets; the engine fetches each selected
language blob lazily and caches its loaded grammar in the browser. It exposes no playground
mutation endpoint. Source is capped at 64 KiB in the engine and is rendered back with `textContent`,
never HTML. The production browser gate verifies all 206 languages, proves a second grammar is
fetched on demand, and fails if editor interaction emits a non-GET request, leaks its private marker
into a URL, or causes a full document navigation.

## License

This documentation site is available under the [MIT License](LICENSE).

## Performance claims

User-visible numbers must match gotreesitter's tagged
[`BENCH.md`](https://github.com/odvcencio/gotreesitter/blob/v0.54.0/BENCH.md). In particular, the
withdrawn 1.54 ms no-tree diagnostic must never be presented as materialized full-parse
performance.
