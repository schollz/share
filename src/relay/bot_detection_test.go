package relay

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestBotDetection tests that bot user agents receive modified HTML with websocket disabled
func TestBotDetection(t *testing.T) {
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create a simple test HTML file
	testHTML := []byte(`<!DOCTYPE html>
<html>
<head>
    <title>Test</title>
</head>
<body>
    <div id="root"></div>
</body>
</html>`)

	// Create handler with mock filesystem
	handler := spaHandler{
		staticFS:      &mockHTTPFS{content: testHTML},
		installScript: []byte("install script"),
	}

	// Test cases with bot and non-bot user agents
	testCases := []struct {
		name           string
		userAgent      string
		expectBotFlag  bool
		path           string
		description    string
	}{
		{
			name:          "Googlebot",
			userAgent:     "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			expectBotFlag: true,
			path:          "/",
			description:   "Google's web crawler should be detected as bot",
		},
		{
			name:          "Bingbot",
			userAgent:     "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
			expectBotFlag: true,
			path:          "/",
			description:   "Bing's web crawler should be detected as bot",
		},
		{
			name:          "Chrome Browser",
			userAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			expectBotFlag: false,
			path:          "/",
			description:   "Regular Chrome browser should not be detected as bot",
		},
		{
			name:          "Firefox Browser",
			userAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:89.0) Gecko/20100101 Firefox/89.0",
			expectBotFlag: false,
			path:          "/",
			description:   "Regular Firefox browser should not be detected as bot",
		},
		{
			name:          "Twitterbot",
			userAgent:     "Twitterbot/1.0",
			expectBotFlag: true,
			path:          "/some-room",
			description:   "Twitter's crawler should be detected as bot",
		},
		{
			name:          "facebookexternalhit",
			userAgent:     "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)",
			expectBotFlag: true,
			path:          "/",
			description:   "Facebook's crawler should be detected as bot",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			req.Header.Set("User-Agent", tc.userAgent)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			resp := rec.Result()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			// Check if the bot flag is present
			hasBotFlag := bytes.Contains(body, []byte("window.__DISABLE_WEBSOCKET=true"))

			if tc.expectBotFlag && !hasBotFlag {
				t.Errorf("%s: Expected bot flag in HTML for user agent: %s", tc.description, tc.userAgent)
			}

			if !tc.expectBotFlag && hasBotFlag {
				t.Errorf("%s: Did not expect bot flag in HTML for user agent: %s", tc.description, tc.userAgent)
			}

			// Ensure HTML is still served
			if !bytes.Contains(body, []byte("<html>")) {
				t.Errorf("%s: Response should contain HTML", tc.description)
			}
		})
	}
}

// TestBotDetectionStaticFiles tests that static files (CSS, JS) are served normally even for bots
func TestBotDetectionStaticFiles(t *testing.T) {
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	testHTML := []byte(`<!DOCTYPE html><html><head></head><body></body></html>`)
	testCSS := []byte(`body { color: red; }`)
	handler := spaHandler{
		staticFS:      &mockHTTPFSWithStatic{htmlContent: testHTML, cssContent: testCSS},
		installScript: []byte("install"),
	}

	// Test that static files are served normally (not modified) for bots
	req := httptest.NewRequest("GET", "/assets/style.css", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Static files should not have bot flag injected
	if bytes.Contains(body, []byte("window.__DISABLE_WEBSOCKET=true")) {
		t.Error("Static files should not have bot flag injected")
	}

	// Should receive the CSS content
	if !bytes.Contains(body, []byte("body { color: red; }")) {
		t.Error("Should receive CSS content for static file request")
	}
}

// TestCurlHandling tests that curl requests still get the install script
func TestCurlHandling(t *testing.T) {
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	testHTML := []byte(`<!DOCTYPE html><html><head></head><body></body></html>`)
	installScript := []byte("#!/bin/bash\necho 'install script'")
	handler := spaHandler{
		staticFS:      &mockHTTPFS{content: testHTML},
		installScript: installScript,
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Should get install script for curl
	if !bytes.Equal(body, installScript) {
		t.Error("Curl requests should receive install script")
	}

	// Should not contain HTML
	if bytes.Contains(body, []byte("<html>")) {
		t.Error("Curl requests should not receive HTML")
	}
}

// mockHTTPFS is a simple mock implementation of http.FileSystem for testing
type mockHTTPFS struct {
	content []byte
}

func (m *mockHTTPFS) Open(name string) (http.File, error) {
	// Return mock file for index.html
	if strings.Contains(name, "index.html") || name == "/" {
		return &mockHTTPFile{
			content: m.content,
			name:    name,
		}, nil
	}
	// For other files, return "not found"
	return nil, os.ErrNotExist
}

// mockHTTPFSWithStatic extends mockHTTPFS to serve static files
type mockHTTPFSWithStatic struct {
	htmlContent []byte
	cssContent  []byte
}

func (m *mockHTTPFSWithStatic) Open(name string) (http.File, error) {
	// Return mock file for index.html
	if strings.Contains(name, "index.html") || name == "/" {
		return &mockHTTPFile{
			content: m.htmlContent,
			name:    name,
		}, nil
	}
	// Return mock CSS file
	if strings.Contains(name, ".css") {
		return &mockHTTPFile{
			content: m.cssContent,
			name:    name,
		}, nil
	}
	// For other files, return "not found"
	return nil, os.ErrNotExist
}

// mockHTTPFile implements http.File interface
type mockHTTPFile struct {
	content []byte
	reader  *bytes.Reader
	name    string
}

func (m *mockHTTPFile) Close() error {
	return nil
}

func (m *mockHTTPFile) Read(p []byte) (n int, err error) {
	if m.reader == nil {
		m.reader = bytes.NewReader(m.content)
	}
	return m.reader.Read(p)
}

func (m *mockHTTPFile) Seek(offset int64, whence int) (int64, error) {
	if m.reader == nil {
		m.reader = bytes.NewReader(m.content)
	}
	return m.reader.Seek(offset, whence)
}

func (m *mockHTTPFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, nil
}

func (m *mockHTTPFile) Stat() (os.FileInfo, error) {
	return &mockFileInfo{
		name: m.name,
		size: int64(len(m.content)),
	}, nil
}

// mockFileInfo implements os.FileInfo
type mockFileInfo struct {
	name string
	size int64
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *mockFileInfo) IsDir() bool        { return false }
func (m *mockFileInfo) Sys() interface{}   { return nil }
