package docs

import (
	"log"

	"m31labs.dev/gosx/route"
	"m31labs.dev/gosx/server"
)

var docsPublicAssetURL func(string) string

func BindPublicAssetURL(fn func(string) string) {
	docsPublicAssetURL = fn
}

func PublicAssetURL(path string) string {
	if docsPublicAssetURL != nil {
		return docsPublicAssetURL(path)
	}
	return server.AssetURL(path)
}

func RegisterDocsPage(title, description string, opts route.FileModuleOptions) {
	metadata := opts.Metadata
	opts.Metadata = func(ctx *route.RouteContext, page route.FilePage, data any) (server.Metadata, error) {
		meta := server.Metadata{
			Title:       server.Title{Default: title + " | GoTreeSitter Docs"},
			Description: description,
		}
		if metadata == nil {
			return meta, nil
		}
		extra, err := metadata(ctx, page, data)
		if err != nil {
			return server.Metadata{}, err
		}
		return mergeDocsMetadata(meta, extra), nil
	}
	if err := route.RegisterFileModuleCaller(1, opts); err != nil {
		log.Fatal(err)
	}
}

func RegisterStaticDocsPage(title, description string, opts route.FileModuleOptions) {
	metaMetadata := opts.Metadata
	opts.Metadata = func(ctx *route.RouteContext, page route.FilePage, data any) (server.Metadata, error) {
		meta := server.Metadata{
			Title:       server.Title{Default: title + " | GoTreeSitter Docs"},
			Description: description,
		}
		if metaMetadata == nil {
			return meta, nil
		}
		extra, err := metaMetadata(ctx, page, data)
		if err != nil {
			return server.Metadata{}, err
		}
		return mergeDocsMetadata(meta, extra), nil
	}
	if err := route.RegisterFileModuleCaller(1, opts); err != nil {
		log.Fatal(err)
	}
}

func mergeDocsMetadata(base, extra server.Metadata) server.Metadata {
	if extra.Title.Default != "" || extra.Title.Absolute != "" {
		base.Title = extra.Title
	}
	if extra.Description != "" {
		base.Description = extra.Description
	}
	if len(extra.Links) > 0 {
		base.Links = append(base.Links, extra.Links...)
	}
	if extra.MetadataBase != "" {
		base.MetadataBase = extra.MetadataBase
	}
	if extra.Alternates != nil {
		base.Alternates = extra.Alternates
	}
	if extra.Robots != nil {
		base.Robots = extra.Robots
	}
	if extra.Icons != nil {
		base.Icons = extra.Icons
	}
	if extra.Manifest != "" {
		base.Manifest = extra.Manifest
	}
	if extra.Verification != nil {
		base.Verification = extra.Verification
	}
	if len(extra.ThemeColor) > 0 {
		base.ThemeColor = append(base.ThemeColor, extra.ThemeColor...)
	}
	if extra.OpenGraph != nil {
		base.OpenGraph = extra.OpenGraph
	}
	if extra.Twitter != nil {
		base.Twitter = extra.Twitter
	}
	if len(extra.JSONLD) > 0 {
		base.JSONLD = append(base.JSONLD, extra.JSONLD...)
	}
	if len(extra.Other) > 0 {
		base.Other = append(base.Other, extra.Other...)
	}
	return base
}
