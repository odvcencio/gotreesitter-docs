package playgroundengine

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/odvcencio/gotreesitter/grammars"
	grammarruntime "github.com/odvcencio/gotreesitter/grammars/runtime"
)

func TestVerifyFetchedGrammarBlob(t *testing.T) {
	data := []byte("grammar blob")
	sum := sha256.Sum256(data)
	url := fmt.Sprintf("/playground/grammars/sql.%x.bin", sum[:6])
	if err := VerifyFetchedGrammarBlob(url, len(data), data); err != nil {
		t.Fatalf("valid fetched grammar rejected: %v", err)
	}

	tampered := append([]byte(nil), data...)
	tampered[0] ^= 1
	if err := VerifyFetchedGrammarBlob(url, len(tampered), tampered); err == nil {
		t.Fatal("fetched grammar with a mismatched content hash was accepted")
	}
	if err := VerifyFetchedGrammarBlob(url, len(data)+1, data); err == nil {
		t.Fatal("fetched grammar with an unexpected byte count was accepted")
	}
}

func TestFetchedYAMLBlobMatchesRuntimeDecoder(t *testing.T) {
	const childEnv = "PLAYGROUNDENGINE_FETCHED_YAML_TEST_CHILD"
	if os.Getenv(childEnv) != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestFetchedYAMLBlobMatchesRuntimeDecoder$")
		cmd.Env = append(os.Environ(), childEnv+"=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fetched YAML parser subprocess failed: %v\n%s", err, output)
		}
		return
	}

	const source = "name: gotreesitter\nlanguages: 28\n"
	blob := append([]byte(nil), grammars.BlobByName("yaml")...)
	if len(blob) == 0 {
		t.Fatal("YAML grammar blob is unavailable for the simulated fetch")
	}
	fetched, err := LoadFetchedLanguage("yaml", blob)
	if err != nil {
		t.Fatalf("load fetched YAML grammar: %v", err)
	}
	got := ParseLanguage(source, "", "yaml", fetched, true)

	// Build the control after the fetched parse so cached registry state cannot
	// make a first-use loading bug disappear.
	decoded, err := grammarruntime.DecodeAndCertifyLanguageBlob("yaml", blob)
	if err != nil {
		t.Fatalf("load YAML through the runtime decoder: %v", err)
	}
	want := ParseLanguage(source, "", "yaml", decoded, true)
	if want.ParseError != "" || want.HasErrors || len(want.TreeRows) == 0 {
		t.Fatalf("runtime-decoded YAML control is not clean: %+v", want)
	}
	if got.ParseError != "" || got.HasErrors {
		t.Fatalf("fetched YAML differs from the clean runtime-decoded control: %+v", got)
	}
	if !reflect.DeepEqual(got.TreeRows, want.TreeRows) {
		t.Fatalf("fetched YAML tree differs from the runtime decoder:\ngot  %+v\nwant %+v", got.TreeRows, want.TreeRows)
	}
}

func TestFetchedSQLBlobParsesExternalScannerCases(t *testing.T) {
	const childEnv = "PLAYGROUNDENGINE_FETCHED_SQL_TEST_CHILD"
	if os.Getenv(childEnv) != "1" {
		// LoadFetchedLanguage wraps gotreesitter's process-global aggregate
		// catalog. Isolate this test so it cannot affect other engine tests.
		cmd := exec.Command(os.Args[0], "-test.run=^TestFetchedSQLBlobParsesExternalScannerCases$")
		cmd.Env = append(os.Environ(), childEnv+"=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fetched SQL parser subprocess failed: %v\n%s", err, output)
		}
		return
	}

	// This copy stands in for bytes returned by fetchBytes. The test passes the
	// bytes through the same registration path used before browser parsing.
	fetchedBlob := append([]byte(nil), grammars.BlobByName("sql")...)
	if len(fetchedBlob) == 0 {
		t.Fatal("SQL grammar blob is unavailable for the simulated fetch")
	}
	language, err := LoadFetchedLanguage("sql", fetchedBlob)
	if err != nil {
		t.Fatalf("load fetched SQL grammar: %v", err)
	}

	// Loading one fetched grammar must leave the package catalog available to
	// same-process consumers asking for a different, not-yet-fetched grammar.
	goLanguage := grammarruntime.Language("go")
	if goLanguage == nil || goLanguage.Name != "go" {
		t.Fatalf("load unfetched Go grammar through package catalog: got %#v", goLanguage)
	}

	for _, source := range []string{"SELECT 1;", "SELECT $$x$$;"} {
		t.Run(source, func(t *testing.T) {
			result := ParseLanguage(source, "", "sql", language, false)
			if result.ParseError != "" {
				t.Fatalf("parse error: %s", result.ParseError)
			}
			if result.HasErrors {
				t.Fatal("parse tree contains ERROR or MISSING nodes")
			}
			if len(result.TreeRows) == 0 || result.NodeCount == 0 {
				t.Fatal("parse returned no syntax tree")
			}
		})
	}
}
