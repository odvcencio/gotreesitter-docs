package main

import (
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	docsapp "github.com/odvcencio/gotreesitter-docs/app"
	_ "github.com/odvcencio/gotreesitter-docs/modules"
	"m31labs.dev/gosx"
	"m31labs.dev/gosx/auth"
	"m31labs.dev/gosx/env"
	islandprogram "m31labs.dev/gosx/island/program"
	"m31labs.dev/gosx/route"
	"m31labs.dev/gosx/server"
	"m31labs.dev/gosx/session"
)

type navItem struct {
	href  string
	label string
}

var navItems = []navItem{
	{href: "/", label: "Overview"},
	{href: "/playground", label: "Playground"},
	{href: "/docs/introduction", label: "Introduction"},
	{href: "/docs/getting-started", label: "Getting Started"},
	{href: "/labs/stream", label: "Streaming"},
	{href: "/labs/secret", label: "Secret"},
}

func main() {
	_, thisFile, _, _ := runtime.Caller(0)
	root := server.ResolveAppRoot(thisFile)
	if err := ensureDocsSampleAssets(root); err != nil {
		log.Fatal(err)
	}
	if err := env.LoadDir(root, ""); err != nil {
		log.Fatal(err)
	}
	docsapp.BindPublicAssetURL(func(path string) string {
		return versionedPublicAssetURL(root, path)
	})
	port := getenv("PORT", "8080")
	sessions, err := session.New(getenv("SESSION_SECRET", "gosx-docs-session-secret"), session.Options{})
	if err != nil {
		log.Fatal(err)
	}
	authn := auth.New(sessions, auth.Options{LoginPath: "/docs/auth"})
	docsapp.BindAuth(authn)

	siteLayout, err := route.FileLayout(filepath.Join(root, "app", "layout.gsx"))
	if err != nil {
		log.Fatal(err)
	}
	wrapSite := func(ctx *route.RouteContext, body gosx.Node) gosx.Node {
		return siteLayout(ctx, body)
	}

	router := route.NewRouter()
	router.SetLayout(func(ctx *route.RouteContext, body gosx.Node) gosx.Node {
		// Space Grotesk + JetBrains Mono, per design/GoTreeSitter-Docs.html's
		// <helmet> — the neo-brutalist system's two typefaces. This is the
		// site's actual head (app/layout.gsx only renders the body shell), so
		// the fonts are wired in here rather than in the .gsx layout.
		ctx.AddHead(
			gosx.El("link", gosx.Attrs(gosx.Attr("rel", "preconnect"), gosx.Attr("href", "https://fonts.googleapis.com"))),
			gosx.El("link", gosx.Attrs(gosx.Attr("rel", "preconnect"), gosx.Attr("href", "https://fonts.gstatic.com"), gosx.BoolAttr("crossorigin"))),
			server.Stylesheet("https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@400;500;600;700&family=JetBrains+Mono:ital,wght@0,400;0,500;0,700;1,400&display=swap"),
		)
		ctx.AddHead(server.Stylesheet(docsapp.PublicAssetURL("docs.css")))
		ctx.AddHead(server.NavigationScript())
		return server.HTMLDocument(ctx.Title("GoTreeSitter Docs"), ctx.Head(), body)
	})
	router.Add(route.Route{
		Pattern: "/labs/stream",
		Handler: func(ctx *route.RouteContext) gosx.Node {
			ctx.SetMetadata(server.Metadata{
				Title:       server.Title{Default: "Streaming | GoSX Docs"},
				Description: "Deferred regions flush fallback HTML first, then stream the resolved node into place.",
			})
			region := ctx.DeferWithOptions(server.DeferredOptions{
				Class: "card",
			}, gosx.El("div",
				gosx.El("strong", gosx.Text("Loading region")),
				gosx.El("p", gosx.Text("The server has already flushed this fallback while the deferred resolver finishes.")),
			), func() (gosx.Node, error) {
				time.Sleep(180 * time.Millisecond)
				return gosx.El("div", gosx.Attrs(gosx.Attr("class", "card")),
					gosx.El("strong", gosx.Text("Resolved region")),
					gosx.El("p", gosx.Text("This card streamed after the initial HTML shell and replaced the fallback slot in-place.")),
				), nil
			})

			return wrapSite(ctx, gosx.El("article", gosx.Attrs(gosx.Attr("class", "prose")),
				gosx.El("div", gosx.Attrs(gosx.Attr("class", "page-topper")),
					gosx.El("span", gosx.Attrs(gosx.Attr("class", "eyebrow")), gosx.Text("Streaming")),
					gosx.El("p", gosx.Attrs(gosx.Attr("class", "lede")), gosx.Text("Deferred regions flush fallback HTML first, then stream resolved content into place.")),
				),
				gosx.El("h1", gosx.Text("Streaming in GoSX starts with deferred regions, not a separate rendering stack.")),
				gosx.El("p", gosx.Text("A page can flush its shell immediately, keep the fallback visible, and stream late sections into the live DOM as resolvers finish.")),
				gosx.El("section", gosx.Attrs(gosx.Attr("class", "feature-grid")),
					region,
					gosx.El("div", gosx.Attrs(gosx.Attr("class", "card")),
						gosx.El("strong", gosx.Text("API")),
						gosx.El("p", gosx.Text("Use ctx.Defer(...) or ctx.DeferWithOptions(...) inside server or route handlers.")),
					),
				),
				gosx.El("pre", gosx.Attrs(gosx.Attr("class", "code-block")), gosx.Text(`ctx.Defer(
    <p>Loading...</p>,
    func() (gosx.Node, error) {
        return <section>Resolved</section>, nil
    },
)`)),
				gosx.El("div", gosx.Attrs(gosx.Attr("class", "hero-actions")),
					gosx.El("a", gosx.Attrs(gosx.Attr("href", "/docs/runtime"), gosx.Attr("data-gosx-link", true), gosx.Attr("class", "cta-link")), gosx.Text("Back to runtime")),
					gosx.El("a", gosx.Attrs(gosx.Attr("href", "/"), gosx.Attr("data-gosx-link", true), gosx.Attr("class", "cta-link primary")), gosx.Text("Back to overview")),
				),
			))
		},
	})
	router.Add(route.Route{
		Pattern:    "/labs/secret",
		Middleware: []route.Middleware{authn.Require},
		Handler: func(ctx *route.RouteContext) gosx.Node {
			ctx.SetMetadata(server.Metadata{
				Title:       server.Title{Default: "Secret Lab | GoSX Docs"},
				Description: "A guarded route proving auth middleware works on the same router that serves file-based pages.",
			})
			name := ""
			if user, ok := auth.Current(ctx.Request); ok {
				name = user.Name
			}
			return wrapSite(ctx, gosx.El("article", gosx.Attrs(gosx.Attr("class", "prose")),
				gosx.El("div", gosx.Attrs(gosx.Attr("class", "page-topper")),
					gosx.El("span", gosx.Attrs(gosx.Attr("class", "eyebrow")), gosx.Text("Protected")),
					gosx.El("p", gosx.Attrs(gosx.Attr("class", "lede")), gosx.Text("This route is wrapped in auth middleware before the router resolves the page handler.")),
				),
				gosx.El("h1", gosx.Text("You reached a guarded route.")),
				gosx.El("p", gosx.Text("Current user: "+name)),
				gosx.El("div", gosx.Attrs(gosx.Attr("class", "hero-actions")),
					gosx.El("a", gosx.Attrs(gosx.Attr("href", "/docs/auth"), gosx.Attr("data-gosx-link", true), gosx.Attr("class", "cta-link primary")), gosx.Text("Back to auth")),
					gosx.El("a", gosx.Attrs(gosx.Attr("href", "/"), gosx.Attr("data-gosx-link", true), gosx.Attr("class", "cta-link")), gosx.Text("Back to overview")),
				),
			))
		},
	})

	if err := router.AddDir(filepath.Join(root, "app"), route.FileRoutesOptions{}); err != nil {
		log.Fatal(err)
	}

	app := server.New()
	router.SetRevalidator(app.Revalidator())
	app.EnableISR()
	app.EnableNavigation()
	app.Use(sessions.Middleware)
	app.Use(authn.Middleware)
	// CSRF-protect everything except POST /playground/detect: that endpoint
	// is a stateless, side-effect-free compute call (a bounded parse-race
	// over the request body, app/playground_api.go) — there is no session
	// state to ride, and it is meant to be plain-curl-able.
	app.Use(func(next http.Handler) http.Handler {
		protected := sessions.Protect(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/playground/detect" {
				next.ServeHTTP(w, r)
				return
			}
			protected.ServeHTTP(w, r)
		})
	})
	app.SetPublicDir(filepath.Join(root, "public"))
	// A `gosx build` run stages the real WASM runtime + bootstrap JS the
	// islands need to hydrate into dist/ (build.json + assets/runtime/*);
	// without this, /gosx/wasm_exec.js and /gosx/runtime.wasm 404 and every
	// island stays server-HTML-only. dist/ is build output (gitignored,
	// regenerated by CI/deploy — see .gitignore), so this only takes effect
	// once that build has actually run; plain `go run .` in dev still falls
	// back to server's no-op bootstrap stub exactly as before.
	if info, err := os.Stat(filepath.Join(root, "dist", "build.json")); err == nil && !info.IsDir() {
		app.SetRuntimeRoot(filepath.Join(root, "dist"))
	}
	mountIslandProgram(app, docsapp.LangSearchProgramPath, docsapp.LangSearchProgram())
	mountIslandProgram(app, docsapp.PlaygroundProgramPath, docsapp.PlaygroundProgram())
	app.Redirect("GET /docs", "/docs/introduction", http.StatusTemporaryRedirect)
	// Live playground data plane (app/playground_api.go). The page itself is
	// file-routed (app/playground/page.gsx); its WASM runtime + client assets
	// are static files under public/playground/, staged by
	// scripts/build-playground-wasm.sh. The {name} wildcard carries the .json
	// suffix ("go.json") because ServeMux wildcards must span a full segment.
	app.API("GET /playground/langs.json", docsapp.PlaygroundLangsHandler)
	app.API("GET /playground/lang/{name}", docsapp.PlaygroundLangHandler)
	app.API("POST /playground/detect", docsapp.PlaygroundDetectHandler)
	app.API("GET /api/meta", func(ctx *server.Context) (any, error) {
		ctx.Cache(server.CachePolicy{
			Public:               true,
			MaxAge:               time.Minute,
			StaleWhileRevalidate: 5 * time.Minute,
		})
		ctx.CacheTag("docs-meta")
		pages := make([]map[string]string, 0, len(navItems))
		for _, item := range navItems {
			pages = append(pages, map[string]string{
				"href":  item.href,
				"label": item.label,
			})
		}
		return map[string]any{
			"ok":      true,
			"product": "gosx-docs",
			"version": gosx.Version,
			"pages":   pages,
		}, nil
	})
	app.HandleAPI(server.APIRoute{
		Pattern:    "GET /api/me",
		Middleware: []server.Middleware{authn.Require},
		Handler: func(ctx *server.Context) (any, error) {
			user, _ := auth.Current(ctx.Request)
			return map[string]any{
				"ok":   true,
				"user": user,
			}, nil
		},
	})
	rootHandler, err := router.BuildChecked()
	if err != nil {
		log.Fatal(err)
	}
	app.Mount("/", rootHandler)

	log.Printf("gotreesitter-docs at http://localhost:%s", port)
	log.Fatal(app.ListenAndServe(":" + port))
}

// mountIslandProgram serves a compiled island program's opcode JSON at
// path, matching the pattern GoSX's own examples use (examples/counter,
// examples/hotswap): the client bootstrap fetches this once (prefetched via
// island.Renderer.PreloadHints) and interprets it in the shared WASM VM —
// see app/lang_island.go / app/playground_island.go's SetProgramAsset
// calls, which point at exactly these paths.
func mountIslandProgram(app *server.App, path string, prog *islandprogram.Program) {
	data, err := islandprogram.EncodeJSON(prog)
	if err != nil {
		log.Fatalf("gotreesitter-docs: encode island program %s: %v", path, err)
	}
	app.Mount(path, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = w.Write(data)
	}))
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func versionedPublicAssetURL(root, name string) string {
	base := server.AssetURL(name)
	payload, err := os.ReadFile(filepath.Join(root, "public", filepath.FromSlash(name)))
	if err != nil {
		return base
	}
	sum := sha256.Sum256(payload)
	return base + "?v=" + hex.EncodeToString(sum[:6])
}

func ensureDocsSampleAssets(root string) error {
	publicDir := filepath.Join(root, "public")
	if err := os.MkdirAll(publicDir, 0755); err != nil {
		return err
	}

	sample := filepath.Join(publicDir, "paper-card.png")
	if _, err := os.Stat(sample); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	const width = 1200
	const height = 780
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			base := uint8(228 + (y*18)/height)
			ink := uint8(36 + (x*94)/width)
			img.Set(x, y, color.RGBA{
				R: base,
				G: uint8(216 + (x*20)/width),
				B: uint8(198 + (y*24)/height),
				A: 255,
			})
			if x > 84 && x < width-84 && y > 84 && y < height-84 && (x+y)%17 < 2 {
				img.Set(x, y, color.RGBA{R: ink, G: uint8(74 + (y*32)/height), B: 62, A: 255})
			}
			if x > 180 && x < width-180 && y > 160 && y < 260 {
				img.Set(x, y, color.RGBA{R: 181, G: 91, B: 52, A: 255})
			}
		}
	}

	file, err := os.Create(sample)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}
