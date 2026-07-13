package docs

import (
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"strings"
	"testing"

	"m31labs.dev/gosx/server"
)

func TestResolvePlaygroundGTSVersion(t *testing.T) {
	tests := []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{name: "missing build info", want: "dev"},
		{
			name: "release dependency",
			info: &debug.BuildInfo{Deps: []*debug.Module{{
				Path:    "github.com/odvcencio/gotreesitter",
				Version: "v0.33.0",
			}}},
			want: "v0.33.0",
		},
		{
			name: "versioned replacement",
			info: &debug.BuildInfo{Deps: []*debug.Module{{
				Path:    "github.com/odvcencio/gotreesitter",
				Version: "v0.33.0",
				Replace: &debug.Module{Path: "github.com/example/fork", Version: "v0.33.1"},
			}}},
			want: "v0.33.1",
		},
		{
			name: "versionless local replacement",
			info: &debug.BuildInfo{Deps: []*debug.Module{{
				Path:    "github.com/odvcencio/gotreesitter",
				Version: "v0.33.0",
				Replace: &debug.Module{Path: "../gotreesitter"},
			}}},
			want: "dev",
		},
		{
			name: "development dependency",
			info: &debug.BuildInfo{Deps: []*debug.Module{{
				Path:    "github.com/odvcencio/gotreesitter",
				Version: "(devel)",
			}}},
			want: "dev",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolvePlaygroundGTSVersion(test.info); got != test.want {
				t.Fatalf("version = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPlaygroundReleaseCacheKeyChangesAcrossVersions(t *testing.T) {
	version := "vA"
	app := server.New()
	app.API("GET /data", func(ctx *server.Context) (any, error) {
		cachePlaygroundReleaseData(ctx, version)
		return map[string]any{"version": version}, nil
	})
	handler := app.Build()

	firstRequest := httptest.NewRequest(http.MethodGet, "/data?v=vA", nil)
	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(firstResponse, firstRequest)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("first status = %d", firstResponse.Code)
	}
	firstETag := firstResponse.Header().Get("ETag")
	if firstETag == "" {
		t.Fatal("first response has no ETag")
	}

	version = "vB"
	secondRequest := httptest.NewRequest(http.MethodGet, "/data?v=vA", nil)
	secondRequest.Header.Set("If-None-Match", firstETag)
	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, secondRequest)
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200", secondResponse.Code)
	}
	if !strings.Contains(secondResponse.Body.String(), `"version":"vB"`) {
		t.Fatalf("second body = %q", secondResponse.Body.String())
	}
	if secondETag := secondResponse.Header().Get("ETag"); secondETag == "" || secondETag == firstETag {
		t.Fatalf("second ETag = %q, first = %q", secondETag, firstETag)
	}
}

func TestPlaygroundDevelopmentDataIsNotStored(t *testing.T) {
	app := server.New()
	app.API("GET /data", func(ctx *server.Context) (any, error) {
		cachePlaygroundReleaseData(ctx, "dev")
		return map[string]any{"version": "dev"}, nil
	})

	request := httptest.NewRequest(http.MethodGet, "/data?v=dev", nil)
	response := httptest.NewRecorder()
	app.Build().ServeHTTP(response, request)
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}
