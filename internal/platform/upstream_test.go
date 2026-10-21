package platform

import (
	"net/http"
	"net/url"
	"testing"
)

func TestUpstreamSelectsProtocolAndRejectsLoops(t *testing.T) {
	c := UpstreamConfig{HTTP: "127.0.0.1:7890", HTTPS: "127.0.0.1:7891"}
	for _, tc := range []struct{ method, url, port string }{{"GET", "http://example.com", "7890"}, {"GET", "https://example.com", "7891"}, {"CONNECT", "https://example.com:443", "7891"}} {
		u, _ := url.Parse(tc.url)
		upstream, err := c.Resolve(&http.Request{Method: tc.method, URL: u})
		if err != nil || upstream.Port() != tc.port {
			t.Fatalf("upstream %v: %v", upstream, err)
		}
	}
	c.HTTP = "localhost:9080"
	u, _ := url.Parse("http://example.com")
	if _, err := c.Resolve(&http.Request{URL: u}); err == nil {
		t.Fatal("recursive proxy was accepted")
	}
}
