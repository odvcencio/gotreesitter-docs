package docs

// Server side of the live /playground page (app/playground/): three JSON
// endpoints mounted in main.go.
//
//	GET  /playground/langs.json       — every registered grammar with its
//	                                    compiled-blob size (0 when a grammar
//	                                    has no embedded blob to ship).
//	GET  /playground/lang/{name}.json — one grammar's compiled blob (base64)
//	                                    plus its highlight query; the client
//	                                    feeds both to the WASM runtime's
//	                                    gotreesitter.loadBlob.
//	POST /playground/detect           — server-side language detection: a
//	                                    bounded parse-race over a shortlist
//	                                    of popular grammars, scored by
//	                                    ERROR/MISSING density.
//
// Parsing itself never goes through these endpoints — the page parses
// client-side in the WASM runtime built by scripts/build-playground-wasm.sh.

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"m31labs.dev/gosx/server"
)

// PlaygroundGTSVersion reports the gotreesitter module version linked into
// this binary. The client keys blob URLs with it (?v=...) so the immutable
// cache lifetime on /playground/lang/{name}.json stays honest: blobs only
// change when the module version does.
func PlaygroundGTSVersion() string {
	playgroundVersionOnce.Do(func() {
		playgroundVersionVal = "dev"
		info, ok := debug.ReadBuildInfo()
		if !ok {
			return
		}
		for _, dep := range info.Deps {
			if dep.Path != "github.com/odvcencio/gotreesitter" {
				continue
			}
			version := dep.Version
			if dep.Replace != nil && dep.Replace.Version != "" {
				version = dep.Replace.Version
			}
			if version != "" {
				playgroundVersionVal = version
			}
			return
		}
	})
	return playgroundVersionVal
}

var (
	playgroundVersionOnce sync.Once
	playgroundVersionVal  string
)

type playgroundLang struct {
	Name       string   `json:"name"`
	Extensions []string `json:"extensions"`
	BlobBytes  int      `json:"blobBytes"`
}

var (
	playgroundLangsOnce sync.Once
	playgroundLangsVal  []playgroundLang
)

// playgroundLangList enumerates grammars.AllLanguages() once. BlobBytes is
// len(BlobByName): grammargen runtime extensions carry no embedded blob and
// report 0 — the client disables those picker entries since loadBlob has
// nothing to load.
func playgroundLangList() []playgroundLang {
	playgroundLangsOnce.Do(func() {
		all := grammars.AllLanguages()
		list := make([]playgroundLang, 0, len(all))
		for _, entry := range all {
			exts := entry.Extensions
			if exts == nil {
				exts = []string{}
			}
			list = append(list, playgroundLang{
				Name:       entry.Name,
				Extensions: exts,
				BlobBytes:  len(grammars.BlobByName(entry.Name)),
			})
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
		playgroundLangsVal = list
	})
	return playgroundLangsVal
}

// PlaygroundLangsHandler serves GET /playground/langs.json.
func PlaygroundLangsHandler(ctx *server.Context) (any, error) {
	ctx.Cache(server.CachePolicy{
		Public:               true,
		MaxAge:               time.Hour,
		StaleWhileRevalidate: 24 * time.Hour,
	})
	return map[string]any{
		"ok":        true,
		"version":   PlaygroundGTSVersion(),
		"languages": playgroundLangList(),
	}, nil
}

// PlaygroundLangHandler serves GET /playground/lang/{name} where {name} must
// look like "go.json". (The .json suffix lives inside the path value because
// net/http ServeMux wildcards must span a whole segment — a literal
// "{name}.json" pattern is rejected at registration.)
func PlaygroundLangHandler(ctx *server.Context) (any, error) {
	raw := ctx.Request.PathValue("name")
	name, ok := strings.CutSuffix(raw, ".json")
	if !ok || name == "" {
		ctx.SetStatus(http.StatusNotFound)
		return nil, errors.New("not found: expected /playground/lang/{name}.json")
	}
	entry := grammars.DetectLanguageByName(name)
	if entry == nil {
		ctx.SetStatus(http.StatusNotFound)
		return nil, errors.New("unknown language: " + name)
	}
	blob := grammars.BlobByName(entry.Name)
	if len(blob) == 0 {
		ctx.SetStatus(http.StatusNotFound)
		return nil, errors.New("no compiled blob for language: " + entry.Name)
	}
	// Blobs change only with the gotreesitter module version and the client
	// keys requests with ?v=<version>, so an immutable year is safe.
	ctx.Cache(server.CachePolicy{
		Public:    true,
		MaxAge:    365 * 24 * time.Hour,
		Immutable: true,
	})
	return map[string]any{
		"ok":             true,
		"name":           entry.Name,
		"blob":           base64.StdEncoding.EncodeToString(blob),
		"blobBytes":      len(blob),
		"highlightQuery": entry.HighlightQuery,
		"version":        PlaygroundGTSVersion(),
	}, nil
}

// playgroundDetectShortlist is the parse-race candidate set, in PRIOR order:
// when two grammars parse a snippet equally cleanly, the earlier entry wins.
// That ordering encodes the required tie-breaks (javascript before
// typescript, c before cpp — the more permissive grammar of each pair parses
// a subset cleanly) and, generalizing the same idea, pushes the
// parse-anything grammars (sql, html, yaml, markdown) to the very end so
// they only win when nothing structured matched. Every name is verified
// against the registry at init; missing ones are dropped.
var playgroundDetectShortlist = []string{
	"go", "c", "python", "javascript", "json", "rust", "java", "ruby",
	"php", "bash", "typescript", "tsx", "cpp", "csharp", "kotlin", "swift",
	"lua", "haskell", "elixir", "zig", "ocaml", "scala", "css", "toml",
	"dockerfile", "make", "sql", "html", "yaml", "markdown",
}

const (
	playgroundDetectMaxSource   = 4 << 10  // score only the first 4KB
	playgroundDetectMaxBody     = 64 << 10 // request body hard cap
	playgroundDetectTimeoutUs   = 50_000   // per-parse budget (50ms)
	playgroundDetectCandidateSz = 5        // candidates returned
)

type playgroundDetectLang struct {
	entry *grammars.LangEntry
	prior int
}

var (
	playgroundDetectOnce sync.Once
	playgroundDetectVal  []playgroundDetectLang
)

func playgroundDetectCandidates() []playgroundDetectLang {
	playgroundDetectOnce.Do(func() {
		for i, name := range playgroundDetectShortlist {
			entry := grammars.DetectLanguageByName(name)
			if entry == nil {
				continue
			}
			playgroundDetectVal = append(playgroundDetectVal, playgroundDetectLang{entry: entry, prior: i})
		}
	})
	return playgroundDetectVal
}

// init resolves the shortlist against the registry at startup so a rename in
// a future gotreesitter release surfaces as a shorter candidate list, never
// a per-request failure.
func init() {
	playgroundDetectCandidates()
}

// PlaygroundDetectCandidate is one scored parse-race entry.
type PlaygroundDetectCandidate struct {
	Lang       string  `json:"lang"`
	Score      float64 `json:"score"`
	ErrorNodes int     `json:"errorNodes"`
}

// PlaygroundDetectHandler serves POST /playground/detect. Body:
// {"source": "..."}. Response: {ok, best, confidence, candidates}.
func PlaygroundDetectHandler(ctx *server.Context) (any, error) {
	ctx.NoStore()
	body := http.MaxBytesReader(nil, ctx.Request.Body, playgroundDetectMaxBody)
	defer body.Close()
	var req struct {
		Source string `json:"source"`
	}
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		ctx.SetStatus(http.StatusBadRequest)
		return nil, errors.New(`bad request: body must be JSON {"source": string}`)
	}
	src := []byte(req.Source)
	if len(src) == 0 {
		ctx.SetStatus(http.StatusBadRequest)
		return nil, errors.New("bad request: source is empty")
	}
	if len(src) > playgroundDetectMaxSource {
		src = src[:playgroundDetectMaxSource]
	}

	candidates := playgroundDetectCandidates()
	type raceResult struct {
		cand  PlaygroundDetectCandidate
		prior int
		ok    bool
	}
	results := make([]raceResult, len(candidates))
	var wg sync.WaitGroup
	for i, cand := range candidates {
		wg.Add(1)
		go func(i int, cand playgroundDetectLang) {
			defer wg.Done()
			// One misbehaving grammar must not take the race down.
			defer func() { _ = recover() }()
			lang := cand.entry.Language()
			if lang == nil {
				return
			}
			parser := gts.NewParser(lang)
			parser.SetTimeoutMicros(playgroundDetectTimeoutUs)
			tree, err := parser.Parse(src)
			if err != nil || tree == nil {
				return
			}
			defer tree.Release()
			root := tree.RootNode()
			if root == nil {
				return
			}
			total, bad := 0, 0
			countBadNodes(root, &total, &bad)
			if total == 0 {
				return
			}
			results[i] = raceResult{
				cand: PlaygroundDetectCandidate{
					Lang:       cand.entry.Name,
					Score:      float64(bad) / float64(total),
					ErrorNodes: bad,
				},
				prior: cand.prior,
				ok:    true,
			}
		}(i, cand)
	}
	wg.Wait()

	scored := make([]raceResult, 0, len(results))
	for _, res := range results {
		if res.ok {
			scored = append(scored, res)
		}
	}
	if len(scored) == 0 {
		return map[string]any{
			"ok":         true,
			"best":       "",
			"confidence": 0.0,
			"candidates": []PlaygroundDetectCandidate{},
		}, nil
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].cand.Score != scored[j].cand.Score {
			return scored[i].cand.Score < scored[j].cand.Score
		}
		return scored[i].prior < scored[j].prior
	})

	top := make([]PlaygroundDetectCandidate, 0, playgroundDetectCandidateSz)
	for _, res := range scored[:min(len(scored), playgroundDetectCandidateSz)] {
		top = append(top, res.cand)
	}

	// Confidence: how clean the winner parsed, damped when the runner-up is
	// just as clean (ties are decided by prior only, so they deserve less
	// certainty than a clear margin).
	best := scored[0].cand
	cleanliness := 1 - best.Score
	if cleanliness < 0 {
		cleanliness = 0
	}
	separation := 1.0
	if len(scored) > 1 {
		separation = (scored[1].cand.Score - best.Score) * 25
		if separation > 1 {
			separation = 1
		}
	}
	confidence := math.Round(cleanliness*(0.55+0.45*separation)*100) / 100

	return map[string]any{
		"ok":         true,
		"best":       best.Lang,
		"confidence": confidence,
		"candidates": top,
	}, nil
}

// countBadNodes walks every node (named and anonymous) tallying the total
// visited and how many are ERROR or MISSING — the detect race's score is
// bad/total, an error-density normalized by tree size.
func countBadNodes(n *gts.Node, total, bad *int) {
	if n == nil {
		return
	}
	*total++
	if n.IsError() || n.IsMissing() {
		*bad++
	}
	for i := 0; i < n.ChildCount(); i++ {
		countBadNodes(n.Child(i), total, bad)
	}
}
