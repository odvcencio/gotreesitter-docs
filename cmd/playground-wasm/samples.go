package main

import "github.com/odvcencio/gotreesitter-docs/internal/playgroundsamples"

type languageSample = playgroundsamples.Sample

var languageSamples = playgroundsamples.All()

func sampleFor(language string) (languageSample, bool) {
	return playgroundsamples.ForLanguage(language)
}
