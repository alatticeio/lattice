package web

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var assets embed.FS

// RegisterHandlers mounts the embedded SPA into the Gin engine.
//
// Routing rules:
//   - /api/** → skipped (handled by upstream API routes)
//   - static asset exists → served directly from embed.FS
//   - anything else → fallback to index.html (Vue Router takes over)
func RegisterHandlers(r *gin.Engine) {
	sub, _ := fs.Sub(assets, "dist")

	r.NoRoute(func(c *gin.Context) {
		urlPath := c.Request.URL.Path

		// Let registered API routes return their own 404; don't hijack them.
		if strings.HasPrefix(urlPath, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Strip leading slash so fs.Open can find the file.
		filePath := strings.TrimPrefix(urlPath, "/")
		if filePath == "" {
			filePath = "index.html"
		}

		// Check whether the asset actually exists in the embedded FS.
		f, err := sub.Open(filePath)
		if err != nil {
			// SPA fallback: any unknown path is handled by Vue Router.
			filePath = "index.html"
		} else {
			f.Close() //nolint:errcheck
		}

		// Read and serve the file directly to avoid http.FileServer's 301 redirects.
		data, err := fs.ReadFile(sub, filePath)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		contentType := mime.TypeByExtension(path.Ext(filePath))
		if contentType == "" {
			contentType = "text/html; charset=utf-8"
		}

		c.Data(http.StatusOK, contentType, data)
	})
}
