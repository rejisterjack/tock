// Package web holds embedded dashboard templates and static files.
package web

import "embed"

// FS contains templates and static assets.
//
//go:embed templates/*.html static/*
var FS embed.FS
