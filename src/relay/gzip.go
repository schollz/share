package relay

import (
	"io"
	"net/http"
	"strings"
)

// gzipFileHandler wraps an http.Handler to serve pre-compressed .gz files
// when available and the client accepts gzip encoding
type gzipFileHandler struct {
	handler http.Handler
}

func (g gzipFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if this is a curl request - if so, let spaHandler handle it
	// to serve the install script instead of the web UI
	ua := strings.ToLower(r.UserAgent())
	if strings.Contains(ua, "curl") {
		g.handler.ServeHTTP(w, r)
		return
	}

	// Get the path
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	// Set cache headers based on file type
	setCacheHeaders(w, path)

	// Check if client accepts gzip
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		g.handler.ServeHTTP(w, r)
		return
	}

	// Try to open the .gz version from the embedded filesystem
	if fs, ok := g.handler.(spaHandler); ok {
		gzPath := path + ".gz"
		gzFile, err := fs.staticFS.Open(gzPath)
		if err == nil {
			defer gzFile.Close()

			// Get file info for content type detection
			stat, err := gzFile.Stat()
			if err == nil {
				// Set appropriate headers
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Set("Vary", "Accept-Encoding")

				// Detect and set content type based on original file extension
				contentType := detectContentType(path)
				if contentType != "" {
					w.Header().Set("Content-Type", contentType)
				}

				// Serve the gzipped file
				http.ServeContent(w, r, path, stat.ModTime(), gzFile.(io.ReadSeeker))
				return
			}
		}
	}

	// Fall back to original file
	g.handler.ServeHTTP(w, r)
}

// setCacheHeaders sets appropriate cache headers based on the file path
func setCacheHeaders(w http.ResponseWriter, path string) {
	// Assets with hashes in the filename can be cached indefinitely (immutable)
	// Pattern: /assets/filename-[hash].ext
	if strings.HasPrefix(path, "/assets/") && strings.Contains(path, "-") {
		// Check if the file has a hash (contains hyphen before extension)
		lastDot := strings.LastIndex(path, ".")
		lastHyphen := strings.LastIndex(path, "-")
		if lastHyphen > 0 && lastDot > lastHyphen {
			// This appears to be a hashed asset file
			// Cache for 1 year and mark as immutable
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			return
		}
	}

	// HTML files should not be cached (always check for updates)
	if strings.HasSuffix(path, ".html") || path == "/" {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		return
	}

	// Other static assets: cache but revalidate
	w.Header().Set("Cache-Control", "public, max-age=3600, must-revalidate")
}

// detectContentType returns the MIME type based on file extension
func detectContentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".json"):
		return "application/json"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(path, ".gif"):
		return "image/gif"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(path, ".woff"):
		return "font/woff"
	case strings.HasSuffix(path, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(path, ".ttf"):
		return "font/ttf"
	case strings.HasSuffix(path, ".ico"):
		return "image/x-icon"
	default:
		return ""
	}
}

// newGzipFileHandler wraps a handler to serve pre-compressed gzip files
func newGzipFileHandler(handler http.Handler) http.Handler {
	return gzipFileHandler{handler: handler}
}
