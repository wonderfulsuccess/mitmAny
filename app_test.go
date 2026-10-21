package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/lqqyt2423/go-mitmproxy/cert"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	"mitmAny/internal/capture"
	"mitmAny/internal/platform"
)

func TestStopCaptureKeepsExistingDownloadAlive(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(release) }) }
	defer finish()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "first")
		w.(http.Flusher).Flush()
		<-release
		fmt.Fprint(w, "last")
	}))
	defer origin.Close()
	dir := t.TempDir()
	store, err := capture.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	observer := capture.NewObserver(store)
	ca, err := cert.NewSelfSignCAMemory()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := proxy.NewProxy(&proxy.Options{ReadOnlyCapture: true, NewCaFunc: func() (cert.CA, error) { return ca, nil }})
	if err != nil {
		t.Fatal(err)
	}
	engine.SetUpstreamProxy(func(*http.Request) (*url.URL, error) { return nil, nil })
	engine.AddAddon(observer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- engine.Serve(listener) }()
	defer func() { engine.Close(); <-done; observer.Close() }()
	app := &App{engine: engine, listener: listener, store: store, observer: observer, system: platform.NewProxyManager(dir), running: true, recording: true}
	proxyURL, _ := url.Parse("http://" + listener.Addr().String())
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	response, err := client.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	first := make([]byte, 5)
	if _, err = io.ReadFull(response.Body, first); err != nil {
		t.Fatal(err)
	}
	state, err := app.StopCapture()
	if err != nil || state.Running {
		t.Fatal("stop failed", err)
	}
	finish()
	tail, err := io.ReadAll(response.Body)
	if err != nil || string(tail) != "last" {
		t.Fatalf("stop interrupted download: %q, %v", tail, err)
	}
}

func TestAuditHARRoundTripPreservesBinaryAndTrailers(t *testing.T) {
	a := mobileApp(t)
	input := `{"log":{"version":"1.2","entries":[{"request":{"method":"POST","url":"https://example.test/binary","headers":[{"name":"Content-Type","value":"application/octet-stream"}]},"response":{"status":200,"headers":[{"name":"Content-Encoding","value":"unknown"}],"content":{"text":"AP+A/w==","encoding":"base64"}},"_mitmAny":{"requestBodyBase64":"AP+A/w==","responseEncoding":"compressed","responseTrailers":{"Grpc-Status":["7"]}}}]}}`
	if _, err := a.observer.ImportHAR(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	data, err := a.harData()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := capture.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = restored.ImportHAR(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	flow, err := restored.Detail(restored.Batch(0, 0).Flows[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if flow.RequestBody != "AP+A/w==" || flow.ResponseBody != "AP+A/w==" || flow.ResponseEncoding != "compressed" || flow.ResponseTrailers.Get("Grpc-Status") != "7" {
		t.Fatal("HAR round trip changed binary content or lost metadata")
	}
}
