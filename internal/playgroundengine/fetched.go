package playgroundengine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"sync"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	grammarruntime "github.com/odvcencio/gotreesitter/grammars/runtime"
)

var fetchedGrammarBlobs = struct {
	sync.RWMutex
	byName map[string][]byte
}{byName: make(map[string][]byte)}

var fetchedGrammarCatalogOnce sync.Once

// VerifyFetchedGrammarBlob checks both the indexed byte count and the 12-digit
// SHA-256 prefix in the immutable grammar URL before a fetched blob is loaded.
func VerifyFetchedGrammarBlob(assetURL string, expectedBytes int, data []byte) error {
	if len(data) != expectedBytes {
		return fmt.Errorf("grammar blob has %d bytes; expected %d", len(data), expectedBytes)
	}

	assetPath := strings.SplitN(assetURL, "?", 2)[0]
	filename := path.Base(assetPath)
	stem := strings.TrimSuffix(filename, ".bin")
	dot := strings.LastIndexByte(stem, '.')
	if !strings.HasSuffix(filename, ".bin") || dot < 0 {
		return fmt.Errorf("grammar asset URL %q has no content hash", assetURL)
	}
	want := stem[dot+1:]
	if len(want) != 12 {
		return fmt.Errorf("grammar asset URL %q has an invalid content hash", assetURL)
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:6])
	if got != want {
		return fmt.Errorf("grammar blob content hash is %s; URL requires %s", got, want)
	}
	return nil
}

// LoadFetchedLanguage binds a verified, fetched grammar blob to gotreesitter's
// runtime loaders before loading its language or parsing source with it.
func LoadFetchedLanguage(name string, blob []byte) (*gts.Language, error) {
	blobName := strings.TrimSuffix(name, ".bin") + ".bin"
	fetchedGrammarBlobs.Lock()
	fetchedGrammarBlobs.byName[blobName] = blob
	fetchedGrammarBlobs.Unlock()

	// SQL's scanner asks the runtime loader for "sql.bin". RegisterBlob uses
	// that normalized name, and this catalog provides the same fetched bytes
	// because grammars' aggregate catalog otherwise takes precedence over
	// individual providers (and tries the filesystem in external-blob builds).
	// readFetchedGrammarBlob falls back to grammars.BlobByName for other names
	// so consumers sharing this process retain the package catalog behavior.
	grammarruntime.RegisterBlob(blobName, func() []byte { return blob })
	fetchedGrammarCatalogOnce.Do(func() {
		grammarruntime.RegisterCatalog(readFetchedGrammarBlob, canonicalFetchedGrammarName)
	})

	language, err := gts.LoadLanguage(blob)
	if err != nil {
		return nil, err
	}
	language.Name = strings.TrimSuffix(name, ".bin")
	if scanner := grammars.LookupExternalScanner(language.Name); scanner != nil {
		if bound, ok := scanner.(languageBoundExternalScanner); ok {
			language.ExternalScanner = bound.ExternalScannerForLanguage(language)
		} else {
			language.ExternalScanner = scanner
		}
	}
	if states := grammars.LookupExternalLexStates(language.Name); len(states) > 0 {
		language.ExternalLexStates = states
	}
	return language, nil
}

type languageBoundExternalScanner interface {
	ExternalScannerForLanguage(*gts.Language) gts.ExternalScanner
}

func readFetchedGrammarBlob(name string) ([]byte, func(), error) {
	fetchedGrammarBlobs.RLock()
	data, ok := fetchedGrammarBlobs.byName[name]
	fetchedGrammarBlobs.RUnlock()
	if ok {
		return data, nil, nil
	}

	if data := grammars.BlobByName(strings.TrimSuffix(name, ".bin")); len(data) > 0 {
		return data, nil, nil
	}
	return nil, nil, fmt.Errorf("grammar blob %q is not registered as fetched or in the package catalog", name)
}

func canonicalFetchedGrammarName(name string) string {
	if entry := grammars.DetectLanguageByName(name); entry != nil {
		return entry.Name
	}
	return ""
}
