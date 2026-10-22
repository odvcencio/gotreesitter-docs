// Package playgroundshare encodes a playground grammar, source and query into
// URL parameters and decodes them again. It has no syscall/js dependency, so
// the server and the browser engine share one implementation.
//
// URL shape: /playground?lang=<grammar>&s=<payload>[&q=<payload>]. A payload is
// "1." followed by the unpadded base64url form of the raw DEFLATE stream of the
// UTF-8 text. The "1." prefix lets a later version change the encoding without
// breaking old links.
package playgroundshare

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	ParamLang   = "lang"
	ParamSource = "s"
	ParamQuery  = "q"

	// MaxSourceBytes matches the editor's maxlength.
	MaxSourceBytes = 64 << 10
	// MaxQueryBytes bounds the optional query text.
	MaxQueryBytes = 8 << 10
	// MaxParamLength bounds each encoded payload so the whole URL stays under
	// the 8 KiB request-line limit of nginx-class proxies.
	MaxParamLength = 7000

	encodingVersion = "1."
)

// ErrTooLarge reports a payload over the share limits.
var ErrTooLarge = errors.New("playgroundshare: payload exceeds the share limit")

// Share is the state a link carries.
type Share struct {
	Language string
	Source   string
	Query    string
}

// Encode returns URL parameters for s, or ErrTooLarge.
func Encode(s Share) (url.Values, error) {
	if len(s.Source) > MaxSourceBytes || len(s.Query) > MaxQueryBytes {
		return nil, ErrTooLarge
	}
	values, err := encodeUnchecked(s)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{ParamSource, ParamQuery} {
		if len(values.Get(name)) > MaxParamLength {
			return nil, ErrTooLarge
		}
	}
	return values, nil
}

func encodeUnchecked(s Share) (url.Values, error) {
	source, err := pack(s.Source)
	if err != nil {
		return nil, err
	}
	values := url.Values{ParamLang: {s.Language}, ParamSource: {source}}
	if s.Query != "" {
		query, err := pack(s.Query)
		if err != nil {
			return nil, err
		}
		values.Set(ParamQuery, query)
	}
	return values, nil
}

func pack(text string) (string, error) {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err := w.Write([]byte(text)); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return encodingVersion + base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}

// Decode reads a share from values. It returns ok=false, nil when the values
// carry no source parameter. A malformed or oversized payload returns an error.
func Decode(values url.Values) (Share, bool, error) {
	encodedSource := values.Get(ParamSource)
	if encodedSource == "" {
		return Share{}, false, nil
	}
	source, err := unpack(encodedSource, MaxSourceBytes)
	if err != nil {
		return Share{}, false, err
	}
	share := Share{Language: values.Get(ParamLang), Source: source}
	if encodedQuery := values.Get(ParamQuery); encodedQuery != "" {
		query, err := unpack(encodedQuery, MaxQueryBytes)
		if err != nil {
			return Share{}, false, err
		}
		share.Query = query
	}
	return share, true, nil
}

func unpack(payload string, limit int) (string, error) {
	if len(payload) > MaxParamLength {
		return "", ErrTooLarge
	}
	body, ok := strings.CutPrefix(payload, encodingVersion)
	if !ok {
		return "", fmt.Errorf("playgroundshare: unknown payload version")
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return "", fmt.Errorf("playgroundshare: payload is not base64url: %w", err)
	}
	reader := flate.NewReader(bytes.NewReader(raw))
	defer reader.Close()
	text, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil {
		return "", fmt.Errorf("playgroundshare: payload is not a DEFLATE stream: %w", err)
	}
	if len(text) > limit {
		return "", ErrTooLarge
	}
	if !utf8.Valid(text) {
		return "", errors.New("playgroundshare: payload is not valid UTF-8")
	}
	return string(text), nil
}
