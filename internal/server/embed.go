package server

import "embed"

//go:embed dist/*
var adminFS embed.FS
