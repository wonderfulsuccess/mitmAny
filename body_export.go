package main

import (
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"unicode"
)

// Match the bytes we export: capture has normally already decoded Content-Encoding.
func bodyFilename(rawURL string, headers http.Header, encoding string, data []byte, request bool) string {
	stem := "response"
	if request {
		stem = "request"
	}
	if encoding == "compressed" {
		encodings := strings.Split(headers.Get("Content-Encoding"), ",")
		ext := map[string]string{"gzip": ".gz", "x-gzip": ".gz", "br": ".br", "deflate": ".deflate", "zstd": ".zst"}[strings.TrimSpace(strings.ToLower(encodings[len(encodings)-1]))]
		if ext == "" {
			ext = ".bin"
		}
		return stem + ext
	}
	mediaType, _, _ := mime.ParseMediaType(headers.Get("Content-Type"))
	mediaType = strings.ToLower(mediaType)
	if json.Valid(data) {
		mediaType = "application/json"
	}
	if mediaType == "" || mediaType == "application/octet-stream" {
		mediaType, _, _ = mime.ParseMediaType(http.DetectContentType(data))
	}
	ext := map[string]string{
		"application/json": ".json", "text/html": ".html", "application/xhtml+xml": ".html",
		"text/plain": ".txt", "text/css": ".css", "text/csv": ".csv", "text/event-stream": ".txt",
		"application/javascript": ".js", "text/javascript": ".js", "application/xml": ".xml", "text/xml": ".xml",
		"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp", "image/svg+xml": ".svg", "image/avif": ".avif", "image/x-icon": ".ico",
		"application/pdf": ".pdf", "application/zip": ".zip", "application/wasm": ".wasm",
		"application/x-www-form-urlencoded": ".txt", "audio/mpeg": ".mp3", "video/mp4": ".mp4",
	}[mediaType]
	if ext == "" && strings.HasSuffix(mediaType, "+json") {
		ext = ".json"
	}
	if ext == "" && strings.HasSuffix(mediaType, "+xml") {
		ext = ".xml"
	}
	if ext == "" && mediaType != "application/octet-stream" {
		if extensions, _ := mime.ExtensionsByType(mediaType); len(extensions) > 0 {
			ext = extensions[0]
		}
	}
	// Prefer a server filename; never allow it to escape the save dialog directory.
	name := ""
	if !request {
		if _, params, err := mime.ParseMediaType(headers.Get("Content-Disposition")); err == nil {
			name = params["filename"]
		}
		if name == "" {
			if u, err := url.Parse(rawURL); err == nil {
				name = path.Base(u.Path)
			}
		}
	}
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"|?*`, r) {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." || name == "/" || name == ".." {
		name = stem
	}
	oldExt := path.Ext(name)
	if ext == "" {
		ext = oldExt
	}
	if ext == "" {
		ext = ".bin"
	}
	// Keep an equivalent extension (.jpeg etc.), otherwise replace misleading suffixes.
	if oldExt != "" && mime.TypeByExtension(oldExt) != "" {
		oldType, _, _ := mime.ParseMediaType(mime.TypeByExtension(oldExt))
		if oldType == mediaType {
			return name
		}
	}
	return strings.TrimSuffix(name, oldExt) + ext
}
