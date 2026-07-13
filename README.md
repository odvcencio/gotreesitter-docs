# gotreesitter documentation site

The documentation and browser playground for
[`github.com/odvcencio/gotreesitter`](https://github.com/odvcencio/gotreesitter), built with
[GoSX](https://github.com/odvcencio/gosx). Pages render on the server; the language catalog uses a
small GoSX island, and `/playground` loads the released gotreesitter WASM runtime plus grammar blobs
on demand.

Live site: [gotreesitter.m31labs.dev](https://gotreesitter.m31labs.dev/)

## Local development

Requirements:

- Go 1.26
- GoSX 0.29.5 (`go install m31labs.dev/gosx/cmd/gosx@v0.29.5`)
- TinyGo 0.40.1 for the production GoSX runtime

```sh
cp .env.example .env
gosx dev .
```

The interactive playground runtime is generated and intentionally untracked:

```sh
./scripts/build-playground-wasm.sh
```

That script resolves the exact gotreesitter version from `go.mod`, builds `wasm/runtime` with the
standard Go `js/wasm` target, and stages the runtime and compressed sidecar under
`public/playground/`.

## Production verification

Run the complete source, build, export, size, and route smoke gate:

```sh
./scripts/verify-production.sh
```

The gate derives the exact GoSX and gotreesitter versions from `go.mod`, rejects local engine
overrides, validates every `.gsx` source, runs the Go tests, creates a clean `gosx build --prod`
artifact, runs `gosx export`, reports runtime size, boots `dist/run.sh`, and requests every
documentation route plus the playground APIs and browser assets. It also proves that the
release-versioned parser runtime is served from its gzip sidecar with immutable caching. The `gosx`
binary on `PATH` must exactly match the version in `go.mod`.

Browser performance budgets are optional because they require a local Chrome installation:

```sh
RUN_BROWSER_PERF=1 ./scripts/verify-production.sh
```

The public CI workflow installs the exact GoSX and TinyGo versions declared by this repository,
then runs module verification, tests, vet, GoSX source checks, the production build/export, and the
same route, API, asset, compression, and cache-policy gate.

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
Unversioned island, API, and runtime URLs revalidate or use `no-store`, while hashed `/assets/*`
and exact-version playground resources may be cached immutably when the origin says so.

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
[`BENCH.md`](https://github.com/odvcencio/gotreesitter/blob/v0.33.0/BENCH.md). In particular, the
withdrawn 1.54 ms no-tree diagnostic must never be presented as materialized full-parse
performance.
