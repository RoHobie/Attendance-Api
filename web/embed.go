// Package web embeds the dashboard files.
package web

import "embed"

//go:embed index.html style.css app.js
var Files embed.FS
