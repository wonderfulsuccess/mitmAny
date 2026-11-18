package capture

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/cert"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

func testProxy(t testing.TB, intercept bool) (*http.Client, *Store, *Observer) {
	return testProxyOptions(t, intercept, false, nil)
}
func testProxyOptions(t testing.TB, intercept, first bool, upstreamRoots *x509.CertPool, upstreamURLs ...*url.URL) (*http.Client, *Store, *Observer) {
	client, store, observer, _ := testProxyEngine(t, intercept, first, upstreamRoots, upstreamURLs...)
	return client, store, observer
}
func testProxyEngine(t testing.TB, intercept, first bool, upstreamRoots *x509.CertPool, upstreamURLs ...*url.URL) (*http.Client, *Store, *Observer, *proxy.Proxy) {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	observer := NewObserver(store)
	ca, err := cert.NewSelfSignCAMemory()
	if err != nil {
		t.Fatal(err)
	}
	p, err := proxy.NewProxy(&proxy.Options{ReadOnlyCapture: true, UpstreamRootCAs: upstreamRoots, NewCaFunc: func() (cert.CA, error) { return ca, nil }})
	if err != nil {
		t.Fatal(err)
	}
	p.SetUpstreamProxy(func(*http.Request) (*url.URL, error) {
		if len(upstreamURLs) > 0 {
			return upstreamURLs[0], nil
		}
		return nil, nil
	})
	p.AddAddon(proxy.NewUpstreamCertAddon(first))
	p.AddAddon(observer)
	p.SetShouldInterceptRule(func(*http.Request) bool { return intercept })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- p.Serve(listener) }()
	proxyURL, _ := url.Parse("http://" + listener.Addr().String())
	roots := x509.NewCertPool()
	roots.AddCert(ca.GetRootCA())
	tr := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableCompression: true, ForceAttemptHTTP2: true, TLSClientConfig: &tls.Config{RootCAs: roots}, MaxIdleConnsPerHost: 32}
	t.Cleanup(func() { tr.CloseIdleConnections(); p.Close(); <-done; observer.Close() })
	return &http.Client{Transport: tr, Timeout: 5 * time.Second}, store, observer, p
}
func eventually(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("capture did not settle")
}
func TestResponseStreamsBeforeOriginCompletes(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(release) }) }
	defer finish()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, "first")
		w.(http.Flusher).Flush()
		<-release
		io.WriteString(w, "last")
	}))
	defer origin.Close()
	client, store, _ := testProxy(t, true)
	response, err := client.Get(origin.URL + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	first := make([]byte, 5)
	read := make(chan error, 1)
	go func() { _, err := io.ReadFull(response.Body, first); read <- err }()
	select {
	case err := <-read:
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != "first" {
			t.Fatalf("unexpected prefix %q", first)
		}
	case <-time.After(2 * time.Second):
		finish()
		t.Fatal("proxy buffered the response instead of streaming")
	}
	finish()
	rest, err := io.ReadAll(response.Body)
	if err != nil || string(rest) != "last" {
		t.Fatalf("tail %q: %v", rest, err)
	}
	eventually(t, func() bool {
		batch := store.Batch(0, 0)
		if len(batch.Flows) != 1 || !batch.Flows[0].Complete {
			return false
		}
		d, _ := store.Detail(batch.Flows[0].ID)
		data, _ := base64.StdEncoding.DecodeString(d.ResponseBody)
		return string(data) == "firstlast" && d.ResponseSize == 9
	})
}
func TestUploadAndCompressedResponseAreUnchanged(t *testing.T) {
	payload := bytes.Repeat([]byte("upload-body-"), 200000)
	expected := sha256.Sum256(payload)
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	zw.Write([]byte(`{"ok":true,"message":"hello"}`))
	zw.Close()
	received := make(chan bool, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- sha256.Sum256(body) == expected && r.ContentLength == int64(len(payload)) && r.Header.Get("X-Test") == "preserved"
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/json")
		w.Write(compressed.Bytes())
	}))
	defer origin.Close()
	client, store, _ := testProxy(t, true)
	request, _ := http.NewRequest("POST", origin.URL+"/upload", bytes.NewReader(payload))
	request.Header.Set("X-Test", "preserved")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || !bytes.Equal(body, compressed.Bytes()) {
		t.Fatal("compressed bytes changed", err)
	}
	if !<-received {
		t.Fatal("upload body or framing was changed")
	}
	eventually(t, func() bool {
		batch := store.Batch(0, 0)
		if len(batch.Flows) != 1 || !batch.Flows[0].Complete {
			return false
		}
		d, _ := store.Detail(batch.Flows[0].ID)
		preview, _ := base64.StdEncoding.DecodeString(d.ResponseBody)
		return d.Truncated && d.RequestSize == int64(len(payload)) && d.RequestCaptured <= PreviewLimit && string(preview) == `{"ok":true,"message":"hello"}`
	})
}
func TestHTTPSErrorDoesNotDisableUpstreamVerification(t *testing.T) {
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("untrusted upstream must not be reached") }))
	defer origin.Close()
	client, store, _ := testProxy(t, true)
	response, err := client.Get(origin.URL)
	if err == nil {
		response.Body.Close()
		if response.StatusCode != 502 {
			t.Fatalf("unexpected status %d", response.StatusCode)
		}
	}
	eventually(t, func() bool {
		for _, flow := range store.Batch(0, 0).Flows {
			if flow.Error != "" {
				return true
			}
		}
		return false
	})
}
func TestConcurrentRequestsAndPause(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer origin.Close()
	client, store, observer := testProxy(t, true)
	var group sync.WaitGroup
	for i := 0; i < 40; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			response, err := client.Get(origin.URL)
			if err != nil {
				t.Error(err)
				return
			}
			body, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil || string(body) != `{"status":"ok"}` {
				t.Error("corrupt response", err)
			}
		}()
	}
	group.Wait()
	eventually(t, func() bool {
		batch := store.Batch(0, 0)
		if len(batch.Flows) != 40 {
			return false
		}
		for _, f := range batch.Flows {
			if !f.Complete {
				return false
			}
		}
		return true
	})
	observer.SetRecording(false)
	response, err := client.Get(origin.URL + "/paused")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if len(store.Batch(0, 0).Flows) != 40 {
		t.Fatal("pause recorded an extra request")
	}
}
func TestSaturatedQueueNeverBlocksOrChangesBody(t *testing.T) {
	observer := &Observer{queue: make(chan event), blocks: make(chan []byte, 1)}
	observer.blocks <- make([]byte, chunkSize)
	payload := strings.Repeat("hello", 400000)
	state := &flowState{}
	reader := &tapReader{Reader: strings.NewReader(payload), observer: observer, state: state}
	done := make(chan []byte, 1)
	go func() { body, _ := io.ReadAll(reader); done <- body }()
	select {
	case body := <-done:
		if string(body) != payload {
			t.Fatal("body changed")
		}
		if !state.lost.Load() || observer.Dropped() == 0 {
			t.Fatal("queue loss wasn't reported")
		}
	case <-time.After(time.Second):
		t.Fatal("capture backpressure blocked forwarding")
	}
}
func TestClearDoesNotResurrectInFlightRequests(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	st := &flowState{detail: Detail{Summary: Summary{ID: "active", Started: time.Now().UnixMilli()}}}
	store.update(st, true)
	store.Clear()
	store.update(st, false)
	if store.Batch(0, 0).Total != 0 {
		t.Fatal("cleared flow reappeared")
	}
}
func BenchmarkTapSaturated(b *testing.B) {
	payload := bytes.Repeat([]byte("x"), 1024*1024)
	buffer := make([]byte, chunkSize)
	observer := &Observer{queue: make(chan event), blocks: make(chan []byte, 1)}
	observer.blocks <- make([]byte, chunkSize)
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := &tapReader{Reader: bytes.NewReader(payload), observer: observer, state: &flowState{}}
		io.CopyBuffer(io.Discard, reader, buffer)
	}
}

func TestHTTPSAndHTTP2Inspection(t *testing.T) {
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor != 2 {
			t.Errorf("upstream protocol = %s", r.Proto)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"secure":true}`)
	}))
	origin.EnableHTTP2 = true
	origin.StartTLS()
	defer origin.Close()
	roots := x509.NewCertPool()
	roots.AddCert(origin.Certificate())
	client, store, _ := testProxyOptions(t, true, true, roots)
	response, err := client.Get(origin.URL + "/secure")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || string(body) != `{"secure":true}` {
		t.Fatal("HTTPS response", err, string(body))
	}
	if response.ProtoMajor != 2 {
		t.Fatalf("client protocol = %s", response.Proto)
	}
	eventually(t, func() bool {
		batch := store.Batch(0, 0)
		return len(batch.Flows) == 1 && batch.Flows[0].Complete && batch.Flows[0].Status == 200
	})
}
func TestSSEIsForwardedBeforeStreamEnds(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(release) }) }
	defer finish()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-release
		fmt.Fprint(w, "data: last\n\n")
	}))
	defer origin.Close()
	client, store, _ := testProxy(t, true)
	response, err := client.Get(origin.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	prefix := make([]byte, len("data: first\n\n"))
	_, err = io.ReadFull(response.Body, prefix)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		batch := store.Batch(0, 0)
		if len(batch.Flows) != 1 {
			return false
		}
		d, _ := store.Detail(batch.Flows[0].ID)
		return d.ResponseCaptured > 0 && !d.Complete
	})
	finish()
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
}
func TestHARImportPersistsAndRejectsInvalidURLs(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	input := `{"log":{"version":"1.2","entries":[{"startedDateTime":"2026-10-19T00:00:00Z","time":12,"request":{"method":"GET","url":"https://example.com/test","httpVersion":"HTTP/2.0"},"response":{"status":200,"content":{"mimeType":"application/json","size":11,"text":"{\"ok\":true}"}}}]}}`
	n, err := store.ImportHAR(strings.NewReader(input))
	if err != nil || n != 1 {
		t.Fatal(n, err)
	}
	restored, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	batch := restored.Batch(0, 0)
	if batch.Total != 1 {
		t.Fatal("session not restored")
	}
	d, err := restored.Detail(batch.Flows[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := base64.StdEncoding.DecodeString(d.ResponseBody)
	if string(body) != `{"ok":true}` {
		t.Fatalf("imported body %q", body)
	}
	invalid := strings.ReplaceAll(input, "https://example.com/test", "file:///etc/passwd")
	if _, err = store.ImportHAR(strings.NewReader(invalid)); err == nil {
		t.Fatal("accepted invalid URL")
	}
}
func BenchmarkForwarding(b *testing.B) {
	payload := bytes.Repeat([]byte("x"), 1024*1024)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		w.Write(payload)
	}))
	defer origin.Close()
	for _, mode := range []string{"direct", "proxy"} {
		b.Run(mode, func(b *testing.B) {
			client := origin.Client()
			var observer *Observer
			if mode == "proxy" {
				client, _, observer = testProxy(b, true)
			}
			b.SetBytes(int64(len(payload)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				response, err := client.Get(origin.URL)
				if err != nil {
					b.Fatal(err)
				}
				n, err := io.Copy(io.Discard, response.Body)
				response.Body.Close()
				if err != nil || n != int64(len(payload)) {
					b.Fatal("forwarding mismatch", err, n)
				}
			}
			b.StopTimer()
			if observer != nil {
				observer.Close()
				b.ReportMetric(float64(observer.Dropped())/float64(b.N), "capture-drops/op")
			}
		})
	}
}

func TestStreamingTrailersArePreserved(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		if r.Trailer.Get("X-Upload-Final") != "uploaded" {
			t.Errorf("request trailer = %q", r.Trailer.Get("X-Upload-Final"))
		}
		w.Header().Set("Trailer", "Grpc-Status")
		w.Header().Set("Content-Type", "application/grpc")
		w.Write([]byte{0, 0, 0, 0, 0})
		w.Header().Set("Grpc-Status", "0")
	}))
	defer origin.Close()
	client, store, _ := testProxy(t, true)
	req, _ := http.NewRequest("POST", origin.URL, strings.NewReader("payload"))
	req.ContentLength = -1
	req.Trailer = http.Header{"X-Upload-Final": []string{"uploaded"}}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.Trailer.Get("Grpc-Status") != "0" {
		t.Fatalf("response trailer = %q", res.Trailer.Get("Grpc-Status"))
	}
	eventually(t, func() bool {
		batch := store.Batch(0, 0)
		if len(batch.Flows) != 1 || !batch.Flows[0].Complete {
			return false
		}
		detail, err := store.Detail(batch.Flows[0].ID)
		return err == nil && detail.RequestTrailers.Get("X-Upload-Final") == "uploaded" && detail.ResponseTrailers.Get("Grpc-Status") == "0"
	})
}

func TestExistingHTTPProxyCanBeChained(t *testing.T) {
	seen := make(chan string, 1)
	parent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case seen <- r.URL.String():
		default:
		}
		fmt.Fprint(w, "through parent")
	}))
	defer parent.Close()
	parentURL, _ := url.Parse(parent.URL)
	client, _, _ := testProxyOptions(t, true, false, nil, parentURL)
	response, err := client.Get("http://only-via-parent.invalid/check")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || string(body) != "through parent" {
		t.Fatal("upstream body", err, string(body))
	}
	if <-seen != "http://only-via-parent.invalid/check" {
		t.Fatal("absolute proxy URL wasn't preserved")
	}
}

func TestExistingProxyCarriesHTTPSConnect(t *testing.T) {
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "secure parent") }))
	defer origin.Close()
	connected := make(chan string, 1)
	parent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "CONNECT" {
			http.Error(w, "CONNECT required", 405)
			return
		}
		remote, err := net.Dial("tcp", r.Host)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		client, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			remote.Close()
			return
		}
		defer remote.Close()
		defer client.Close()
		fmt.Fprint(client, "HTTP/1.1 200 Connection Established\r\n\r\n")
		select {
		case connected <- r.Host:
		default:
		}
		go func() { io.Copy(remote, client); remote.Close() }()
		io.Copy(client, remote)
	}))
	defer parent.Close()
	roots := x509.NewCertPool()
	roots.AddCert(origin.Certificate())
	parentURL, _ := url.Parse(parent.URL)
	client, _, _ := testProxyOptions(t, true, true, roots, parentURL)
	response, err := client.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || string(body) != "secure parent" {
		t.Fatal("HTTPS chain failed", err, string(body))
	}
	select {
	case <-connected:
	default:
		t.Fatal("HTTPS bypassed original proxy")
	}
}

func TestEvictionIsReportedToIncrementalClients(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < MaxEntries; i++ {
		st := &flowState{detail: Detail{Summary: Summary{ID: fmt.Sprint(i), Started: int64(i)}}}
		store.update(st, true)
	}
	cursor := store.Batch(0, 0).Cursor
	st := &flowState{detail: Detail{Summary: Summary{ID: "imported-old-timestamp", Started: 1}}}
	store.update(st, true)
	batch := store.Batch(cursor, 0)
	if len(batch.Removed) != 1 || batch.Removed[0] != "0" {
		t.Fatalf("eviction not sent: %#v", batch.Removed)
	}
}

// Request detail must distinguish URL-only GETs from actual uploaded bodies.
func TestRequestDetailRetainsBodiesAndURLParameters(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(fmt.Sprintf("https=%v", secure), func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.Copy(io.Discard, r.Body); w.Write([]byte("ok")) })
			var origin *httptest.Server
			if secure {
				origin = httptest.NewTLSServer(handler)
			} else {
				origin = httptest.NewServer(handler)
			}
			defer origin.Close()
			var roots *x509.CertPool
			if secure {
				roots = x509.NewCertPool()
				roots.AddCert(origin.Certificate())
			}
			client, store, _ := testProxyOptions(t, true, false, roots)
			for _, tc := range []struct{ method, mime, body string }{
				{"GET", "", ""},
				{"POST", "application/json", `{"id":9223372036854775807,"text":"正文"}`},
				{"POST", "application/x-www-form-urlencoded", "name=test&tag=a&tag=b"},
				{"POST", "multipart/form-data; boundary=abc", "--abc\r\nContent-Disposition: form-data; name=\"data\"\r\n\r\nhello\r\n--abc--\r\n"},
			} {
				request, _ := http.NewRequest(tc.method, origin.URL+"/api?tag=a&tag=b&empty=", strings.NewReader(tc.body))
				if tc.mime != "" {
					request.Header.Set("Content-Type", tc.mime)
				}
				response, err := client.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				io.Copy(io.Discard, response.Body)
				response.Body.Close()
				eventually(t, func() bool {
					batch := store.Batch(0, 0)
					for _, f := range batch.Flows {
						if f.Method != tc.method || !f.Complete {
							continue
						}
						d, err := store.Detail(f.ID)
						if err != nil {
							continue
						}
						body, err := base64.StdEncoding.DecodeString(d.RequestBody)
						if err == nil && string(body) == tc.body && d.RequestHeaders.Get("Content-Type") == tc.mime {
							u, _ := url.Parse(d.URL)
							return len(u.Query()["tag"]) == 2 && d.RequestSize == int64(len(tc.body)) && d.RequestCaptured == int64(len(tc.body))
						}
					}
					return false
				})
			}
		})
	}
}

func TestGzipHTMLResponseIsDecodedForPreview(t *testing.T) {
	html := []byte("<!doctype html><html><body><h1>压缩响应预览</h1></body></html>")
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(html); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Encoding", "gzip")
		w.Write(compressed.Bytes())
	}))
	defer origin.Close()
	client, store, _ := testProxy(t, true)
	response, err := client.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || !bytes.Equal(raw, compressed.Bytes()) {
		t.Fatal("forwarding changed gzip bytes", err)
	}
	eventually(t, func() bool {
		batch := store.Batch(0, 0)
		if len(batch.Flows) != 1 || !batch.Flows[0].Complete {
			return false
		}
		d, err := store.Detail(batch.Flows[0].ID)
		if err != nil {
			return false
		}
		preview, err := base64.StdEncoding.DecodeString(d.ResponseBody)
		return err == nil && bytes.Equal(preview, html) && d.ResponseEncoding == "base64" && d.ResponseHeaders.Get("Content-Encoding") == "gzip"
	})
	// Incomplete gzip must remain identifiable as compressed bytes.
	_, encoding := bodyForUI(compressed.Bytes()[:compressed.Len()/2], "gzip")
	if encoding != "compressed" {
		t.Fatalf("incomplete gzip returned %q", encoding)
	}
}

func TestLargeHTMLForwardingAndCapturedPrefix(t *testing.T) {
	// Poorly compressible HTML exceeds both the wire capture limit and decoded limit.
	var html bytes.Buffer
	html.WriteString("<!doctype html><html><body>\n")
	random := rand.New(rand.NewSource(42))
	for i := 0; i < 90000; i++ {
		fmt.Fprintf(&html, "<p>%016x%016x</p>\n", random.Uint64(), random.Uint64())
	}
	html.WriteString("<p>END-OF-LARGE-HTML</p></body></html>")
	var zipped bytes.Buffer
	zw := gzip.NewWriter(&zipped)
	zw.Write(html.Bytes())
	zw.Close()
	if zipped.Len() <= PreviewLimit {
		t.Fatal("fixture must exceed wire preview limit")
	}
	for _, compressed := range []bool{false, true} {
		t.Run(fmt.Sprint(compressed), func(t *testing.T) {
			body := html.Bytes()
			if compressed {
				body = zipped.Bytes()
			}
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if compressed {
					w.Header().Set("Content-Encoding", "gzip")
				}
				w.Write(body)
			}))
			defer origin.Close()
			client, store, _ := testProxy(t, true)
			response, err := client.Get(origin.URL)
			if err != nil {
				t.Fatal(err)
			}
			forwarded, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil || !bytes.Equal(forwarded, body) {
				t.Fatal("forwarded body changed", err)
			}
			eventually(t, func() bool {
				batch := store.Batch(0, 0)
				if len(batch.Flows) != 1 || !batch.Flows[0].Complete {
					return false
				}
				d, err := store.Detail(batch.Flows[0].ID)
				if err != nil {
					return false
				}
				raw, err := os.ReadFile(store.bodyPath(d.ID, false))
				if err != nil || !bytes.HasPrefix(body, raw) || len(raw) > HTMLLimit {
					return false
				}
				if d.Truncated {
					return d.CaptureNote != ""
				}
				decoded, _ := base64.StdEncoding.DecodeString(d.ResponseBody)
				return d.ResponseCaptured == int64(len(body)) && d.ResponseEncoding == "base64" && bytes.Equal(decoded, html.Bytes())
			})
			// The exception is HTML-specific; other encoded bodies remain bounded.
			if compressed {
				_, encoding := bodyForUI(body, "gzip")
				if encoding != "compressed" {
					t.Fatal("non-HTML decoding must stay bounded")
				}
			}
		})
	}
}

func TestHTMLConsumesSessionBudget(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store.entries["html"] = &Detail{Summary: Summary{ID: "html", ContentType: "text/html"}, ResponseCaptured: MaxDiskBytes + 1}
	store.order = []string{"html"}
	store.diskBytes = MaxDiskBytes + 1
	store.changed = true
	store.Flush()
	reopened, err := NewStore(store.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.order) != 0 || reopened.diskBytes != 0 {
		t.Fatal("oversized HTML session was not evicted on restore")
	}
	store.trimLocked()
	if len(store.order) != 0 || store.diskBytes != 0 {
		t.Fatal("HTML did not count toward disk budget")
	}
}

func TestHARImportRetainsLargeHTML(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	html := "<html><body>" + strings.Repeat("a", PreviewLimit+100) + "END</body></html>"
	document := fmt.Sprintf(`{"log":{"version":"1.2","entries":[{"request":{"method":"GET","url":"https://example.test/large"},"response":{"status":200,"content":{"mimeType":"text/html","text":%q}}}]}}`, html)
	count, err := store.ImportHAR(strings.NewReader(document))
	if err != nil || count != 1 {
		t.Fatal(count, err)
	}
	d, err := store.Detail(store.Batch(0, 0).Flows[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := base64.StdEncoding.DecodeString(d.ResponseBody)
	if string(body) != html || d.Truncated || store.diskBytes != int64(len(html)) {
		t.Fatal("imported HTML was limited")
	}
}
