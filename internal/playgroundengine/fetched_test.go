package playgroundengine

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/odvcencio/gotreesitter/grammars"
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

func TestFetchedSQLBlobParsesExternalScannerCases(t *testing.T) {
	const childEnv = "PLAYGROUNDENGINE_FETCHED_SQL_TEST_CHILD"
	if os.Getenv(childEnv) != "1" {
		// LoadFetchedLanguage replaces gotreesitter's process-global aggregate
		// catalog with the playground's fetched-blob catalog. Isolate that test
		// so it cannot change the catalog used by the other engine tests.
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
