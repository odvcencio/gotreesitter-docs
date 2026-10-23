package playgroundshare

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	in := Share{Language: "go", Source: "package main\n\nfunc main() {}\n", Query: "(function_declaration) @f"}
	values, err := Encode(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(values.Get(ParamSource), "1.") || values.Get(ParamLang) != "go" {
		t.Fatalf("unexpected encoding: %v", values)
	}
	out, ok, err := Decode(values)
	if err != nil || !ok || out != in {
		t.Fatalf("decode = %+v, %v, %v", out, ok, err)
	}
}

func TestDecodeWithoutSourceIsNotAShare(t *testing.T) {
	if _, ok, err := Decode(url.Values{"lang": {"go"}}); ok || err != nil {
		t.Fatalf("ok=%v err=%v, want false, nil", ok, err)
	}
}

func TestEncodeRejectsOversizedSource(t *testing.T) {
	// Random-looking text does not compress, so 70 KiB exceeds every limit.
	big := strings.Repeat("xq9z!k#", 10*1024)
	if _, err := Encode(Share{Language: "go", Source: big}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

func TestDecodeRejectsMalformedAndBombPayloads(t *testing.T) {
	for name, payload := range map[string]string{
		"garbage":       "1.%%%",
		"wrong-version": "9.AAAA",
		"too-long":      "1." + strings.Repeat("A", MaxParamLength),
	} {
		if _, _, err := Decode(url.Values{ParamLang: {"go"}, ParamSource: {payload}}); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	// 2 MiB of spaces deflates to a few hundred bytes but inflates past MaxSourceBytes.
	values, err := encodeUnchecked(Share{Language: "go", Source: strings.Repeat(" ", 2<<20)})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Decode(values); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("bomb err = %v, want ErrTooLarge", err)
	}
}

func TestRoundTripKeepsAnEmptyQuery(t *testing.T) {
	in := Share{Language: "go", Source: "package main\n", Query: ""}
	values, err := Encode(in)
	if err != nil {
		t.Fatal(err)
	}
	if !values.Has(ParamQuery) {
		t.Fatalf("an empty query must still be encoded: %v", values)
	}
	out, ok, err := Decode(values)
	if err != nil || !ok || out != in {
		t.Fatalf("decode = %+v, %v, %v", out, ok, err)
	}
}
