# GoTreeSitter GopherCon deck

This directory is a self-contained [gosx-slides](https://github.com/M31-Labs/gosx-slides)
app based on the supplied “GopherCon GoTreeSitter Deck v2” design.

Build the `slides` CLI from the neighboring gosx-slides checkout, then run the
deck from the repository root:

```sh
GOWORK=off go build -C ../gosx-slides -o /tmp/gosx-slides ./cmd/slides
/tmp/gosx-slides serve slides --watch
```

Open the printed URL. Use `←` / `→` or Space to navigate, `o` for the overview,
`p` for presenter view, and `f` for fullscreen.

Before presenting:

```sh
/tmp/gosx-slides validate slides --profile conference --strict
/tmp/gosx-slides rehearse slides
```

The first serve stages the GoSX browser runtime under `slides/build/`; that
directory is intentionally ignored. The deck content lives in `deck.md`, the
visual system in `public/deck.css`, and the two live islands in `Benchmark.gsx`
and `Citation.gsx`.
