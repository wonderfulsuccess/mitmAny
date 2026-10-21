package main

import (
	"net/http"
	"testing"
)

func TestBodyFilenameMatchesExportedBytes(t *testing.T) {
	for _, tc := range []struct {
		name, url, contentType, disposition, encoding string
		data                                          []byte
		request                                       bool
		want                                          string
	}{
		{name: "json keeps numeric bytes", url: "https://test/", data: []byte(`{"id":9223372036854775807}`), want: "response.json"},
		{name: "html sniff", url: "https://test/view.php?q=1", data: []byte("<!DOCTYPE html><html></html>"), want: "view.html"},
		{name: "PNG", url: "https://test/image", contentType: "image/png", want: "image.png"},
		{name: "JPEG suffix", url: "https://test/photo.jpeg", contentType: "image/jpeg", want: "photo.jpeg"},
		{name: "attachment utf8", url: "https://test/download", contentType: "application/pdf", disposition: "attachment; filename*=UTF-8''%E6%8A%A5%E5%91%8A.pdf", want: "报告.pdf"},
		{name: "unsafe filename", url: "https://test/download", contentType: "text/html", disposition: `attachment; filename="../../secret.exe"`, want: "secret.html"},
		{name: "SVG", url: "https://test/icon", contentType: "image/svg+xml", want: "icon.svg"},
		{name: "request", url: "https://test/api", contentType: "application/problem+json", request: true, want: "request.json"},
		{name: "unknown attachment", url: "https://test/blob", contentType: "application/octet-stream", disposition: `attachment; filename="data.custom"`, data: []byte{0, 255, 0}, want: "data.custom"},
		{name: "undecoded gzip", url: "https://test/api", contentType: "application/json", encoding: "compressed", want: "response.gz"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{"Content-Type": {tc.contentType}, "Content-Disposition": {tc.disposition}, "Content-Encoding": {"gzip"}}
			got := bodyFilename(tc.url, h, tc.encoding, tc.data, tc.request)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
