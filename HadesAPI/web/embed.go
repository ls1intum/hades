// Package web embeds the built dashboard single-page app and serves it from the
// HadesAPI Gin server. The real assets are produced by `make ui-build` (Vite)
// into ./dist; a placeholder index.html is committed so the Go module always
// compiles even before a UI build.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var distFS embed.FS

// FS returns the embedded dist directory as a filesystem rooted at its contents.
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// dist is embedded at build time; this can only fail on a broken build.
		panic("web: embedded dist directory missing: " + err.Error())
	}
	return sub
}

// Register serves the embedded SPA via Gin's NoRoute handler so it can never
// shadow explicitly registered routes (/ping, /build, /api/*, /swagger).
// Existing static assets are served directly; any other non-API path falls back
// to index.html for client-side routing.
func Register(r *gin.Engine) {
	fsys := FS()
	fileServer := http.FileServer(http.FS(fsys))

	r.NoRoute(func(c *gin.Context) {
		reqPath := c.Request.URL.Path

		// The SPA must never answer for the API namespace.
		if strings.HasPrefix(reqPath, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		name := strings.TrimPrefix(reqPath, "/")
		if name == "" {
			name = "index.html"
		}
		if f, err := fsys.Open(name); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// Unknown path: serve the SPA entrypoint for client-side routing.
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
