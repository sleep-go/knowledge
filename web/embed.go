package web

import (
	"embed"
)

//go:embed index.html static/*
var StaticFiles embed.FS
