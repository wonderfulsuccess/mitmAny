package capture

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/klauspost/compress/zstd"
)

func TestAuditBrokenResponseIsReported(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Length: 100\r\n\r\nshort")
	}))
	defer origin.Close()
	client, store, _ := testProxy(t, true)
	res, err := client.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := io.ReadAll(res.Body)
	res.Body.Close()
	if string(data) != "short" || readErr == nil {
		t.Fatalf("truncated stream not propagated: %q %v", data, readErr)
	}
	eventually(t, func() bool { b := store.Batch(0, 0); return len(b.Flows) == 1 && b.Flows[0].Complete })
	d, _ := store.Detail(store.Batch(0, 0).Flows[0].ID)
	if d.Error == "" {
		t.Fatal("broken response recorded as successful")
	}
}

func TestAuditRejectedClientCertificateIsVisible(t *testing.T) {
	origin := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer origin.Close()
	roots := x509.NewCertPool()
	roots.AddCert(origin.Certificate())
	client, store, _ := testProxyOptions(t, true, true, roots)
	tr := client.Transport.(*http.Transport)
	tr.TLSClientConfig = &tls.Config{RootCAs: x509.NewCertPool()}
	if res, err := client.Get(origin.URL); err == nil {
		res.Body.Close()
		t.Fatal("untrusted MITM certificate accepted")
	}
	eventually(t, func() bool {
		for _, f := range store.Batch(0, 0).Flows {
			if f.Method == "CONNECT" && f.Error != "" {
				return true
			}
		}
		return false
	})
}

func TestAuditWebSocketHandshakeAndEcho(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(fmt.Sprint("tls=", secure), func(t *testing.T) {
			upgrader := websocket.Upgrader{Subprotocols: []string{"audit.v1"}, CheckOrigin: func(*http.Request) bool { return true }}
			origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-only" || r.Header.Get("Cookie") != "audit=ok" {
					w.WriteHeader(401)
					return
				}
				conn, err := upgrader.Upgrade(w, r, http.Header{"Set-Cookie": []string{"reply=ok"}})
				if err != nil {
					return
				}
				defer conn.Close()
				for {
					kind, data, err := conn.ReadMessage()
					if err != nil {
						return
					}
					if err = conn.WriteMessage(kind, data); err != nil {
						return
					}
				}
			}))
			if secure {
				origin.StartTLS()
			} else {
				origin.Start()
			}
			defer origin.Close()
			roots := x509.NewCertPool()
			if secure {
				roots.AddCert(origin.Certificate())
			}
			client, store, _ := testProxyOptions(t, true, true, roots)
			tr := client.Transport.(*http.Transport)
			dialer := websocket.Dialer{Proxy: tr.Proxy, TLSClientConfig: tr.TLSClientConfig, Subprotocols: []string{"audit.v1"}, HandshakeTimeout: time.Second}
			target := strings.Replace(origin.URL, "http", "ws", 1)
			conn, res, err := dialer.Dial(target, http.Header{"Authorization": []string{"Bearer test-only"}, "Cookie": []string{"audit=ok"}})

			if err != nil {
				t.Fatal("WebSocket handshake failed", err)
			}
			defer conn.Close()
			if conn.Subprotocol() != "audit.v1" || res.Header.Get("Set-Cookie") != "reply=ok" {
				t.Fatal("WebSocket handshake headers lost", conn.Subprotocol(), res.Header)
			}
			payload := bytes.Repeat([]byte("stream-echo-"), 32768)
			if err = conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
				t.Fatal(err)
			}
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, got, err := conn.ReadMessage()
			if err != nil || !bytes.Equal(got, payload) {
				t.Fatal("WebSocket message changed", err, len(got))
			}
			eventually(t, func() bool {
				for _, f := range store.Batch(0, 0).Flows {
					if f.Status == 101 && !f.Complete {
						return true
					}
				}
				return false
			})
			conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
			conn.Close()
			eventually(t, func() bool {
				for _, f := range store.Batch(0, 0).Flows {
					if f.Status == 101 && f.Complete {
						return true
					}
				}
				return false
			})
		})
	}
}

func TestAuditHopByHopHeaders(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Hop") != "" || r.Header.Get("Proxy-Authorization") != "" {
			t.Error("hop-by-hop request header reached origin")
		}
		w.Header().Set("Connection", "X-Origin-Hop")
		w.Header().Set("X-Origin-Hop", "private-hop")
		fmt.Fprint(w, "ok")
	}))
	defer origin.Close()
	client, _, _ := testProxy(t, true)
	req, _ := http.NewRequest("GET", origin.URL, nil)
	req.Header.Set("Connection", "X-Hop")
	req.Header.Set("X-Hop", "private-hop")
	req.Header.Set("Proxy-Authorization", "Basic test-only")
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)
	if res.Header.Get("X-Origin-Hop") != "" {
		t.Fatal("hop-by-hop response header leaked")
	}
}

func TestAuditBodyContentCodings(t *testing.T) {
	plain := []byte(`{"audit":"compressed payload"}`)
	gzipData := func(data []byte) []byte {
		var b bytes.Buffer
		w := gzip.NewWriter(&b)
		w.Write(data)
		w.Close()
		return b.Bytes()
	}
	z, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	for _, tc := range []struct {
		name     string
		data     []byte
		encoding string
	}{
		{"gzip", gzipData(plain), "gzip"},
		{"mixed-case", gzipData(plain), " GZip "},
		{"zstd", z.EncodeAll(plain, nil), "zstd"},
		{"stacked", z.EncodeAll(gzipData(plain), nil), "gzip, zstd"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, encoding := bodyForUI(tc.data, tc.encoding)
			decoded, _ := base64.StdEncoding.DecodeString(body)
			if encoding != "base64" || !bytes.Equal(decoded, plain) {
				t.Fatalf("preview not decoded: encoding=%s bytes=%d", encoding, len(decoded))
			}
		})
	}
}

func TestAuditHTTP2ConcurrentStreams(t *testing.T) {
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"h2":true}`)
	}))
	origin.EnableHTTP2 = true
	origin.StartTLS()
	defer origin.Close()
	roots := x509.NewCertPool()
	roots.AddCert(origin.Certificate())
	client, store, _ := testProxyOptions(t, true, true, roots)
	res, err := client.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := client.Get(origin.URL)
			if err != nil {
				t.Error(err)
				return
			}
			defer res.Body.Close()
			data, err := io.ReadAll(res.Body)
			if err != nil || res.ProtoMajor != 2 || string(data) != `{"h2":true}` {
				t.Errorf("H2 response: %s %q %v", res.Proto, data, err)
			}
		}()
	}
	wg.Wait()
	eventually(t, func() bool {
		batch := store.Batch(0, 0)
		if len(batch.Flows) != 31 {
			return false
		}
		for _, f := range batch.Flows {
			if !f.Complete || f.Error != "" {
				return false
			}
		}
		return true
	})
}

func BenchmarkAuditHTTPS(b *testing.B) {
	payload := bytes.Repeat([]byte("x"), 64*1024)
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		w.Write(payload)
	}))
	origin.EnableHTTP2 = true
	origin.StartTLS()
	defer origin.Close()
	roots := x509.NewCertPool()
	roots.AddCert(origin.Certificate())
	for _, mode := range []string{"direct", "proxy", "proxy-paused"} {
		b.Run(mode, func(b *testing.B) {
			client := origin.Client()
			var observer *Observer
			if mode != "direct" {
				client, _, observer = testProxyOptions(b, true, true, roots)
				if mode == "proxy-paused" {
					observer.SetRecording(false)
				}
			}
			res, err := client.Get(origin.URL)
			if err != nil {
				b.Fatal(err)
			}
			io.Copy(io.Discard, res.Body)
			res.Body.Close()
			b.SetBytes(int64(len(payload)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				res, err := client.Get(origin.URL)
				if err != nil {
					b.Fatal(err)
				}
				n, err := io.Copy(io.Discard, res.Body)
				res.Body.Close()
				if err != nil || n != int64(len(payload)) {
					b.Fatal(n, err)
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

func BenchmarkAuditBodyStore(b *testing.B) {
	store, err := NewStore(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	data := bytes.Repeat([]byte("x"), chunkSize)
	b.ReportAllocs()
	b.SetBytes(PreviewLimit)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state := &flowState{detail: Detail{Summary: Summary{ID: fmt.Sprint(i)}}}
		store.update(state, true)
		for offset := 0; offset < PreviewLimit; offset += chunkSize {
			store.append(state, false, data)
		}
	}
}

func TestAuditCloseStopsHTTPSConnections(t *testing.T) {
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "active") }))
	origin.EnableHTTP2 = true
	origin.StartTLS()
	defer origin.Close()
	roots := x509.NewCertPool()
	roots.AddCert(origin.Certificate())
	client, _, _, engine := testProxyEngine(t, true, true, roots)
	res, err := client.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.ProtoMajor != 2 {
		t.Fatal("expected HTTP/2 tunnel")
	}
	if err = engine.Close(); err != nil {
		t.Fatal(err)
	}
	client.Timeout = time.Second
	res, err = client.Get(origin.URL)
	if err == nil {
		res.Body.Close()
		t.Fatal("closed proxy still forwards through an existing HTTP/2 tunnel")
	}
}

func TestAuditCompressionFailurePreservesRawBytes(t *testing.T) {
	var encoded bytes.Buffer
	writer := gzip.NewWriter(&encoded)
	writer.Write(bytes.Repeat([]byte("a"), PreviewLimit+1))
	writer.Close()
	cases := []struct {
		data     []byte
		encoding string
	}{
		{[]byte("invalid-gzip"), "gzip"}, {encoded.Bytes(), "gzip"},
		{[]byte("unknown coding"), "not-supported"}, {[]byte("too many layers"), "gzip,gzip,gzip,gzip,gzip"},
	}
	for _, tc := range cases {
		body, encoding := bodyForUI(tc.data, tc.encoding)
		raw, _ := base64.StdEncoding.DecodeString(body)
		if encoding != "compressed" || !bytes.Equal(raw, tc.data) {
			t.Fatal("failed decoding replaced raw bytes", tc.encoding)
		}
	}
}

func TestAuditPreviewKeepsContiguousPrefix(t *testing.T) {
	// Lose the first chunk, then make queue space: later data must not masquerade as a prefix.
	observer := &Observer{queue: make(chan event, 1), blocks: make(chan []byte, 1)}
	observer.blocks <- make([]byte, chunkSize)
	observer.queue <- event{}
	state := &flowState{}
	reader := &tapReader{Reader: strings.NewReader("firstsecond"), observer: observer, state: state}
	buf := make([]byte, 5)
	reader.Read(buf)
	<-observer.queue
	reader.Read(buf)
	if len(observer.queue) != 0 || !state.lost.Load() || state.responseBytes.Load() != 10 {
		t.Fatal("preview continued after losing prefix")
	}
}

func TestAuditStalledTLSHandshakeTimesOut(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	stopped := make(chan struct{})
	defer close(stopped)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		<-stopped
	}()
	client, store, _, engine := testProxyEngine(t, true, true, nil)
	engine.Opts.TLSHandshakeTimeout = 50 * time.Millisecond
	started := time.Now()
	res, err := client.Get("https://" + listener.Addr().String())
	if err == nil {
		res.Body.Close()
		if res.StatusCode != 502 {
			t.Fatal(res.StatusCode)
		}
	}
	if time.Since(started) > time.Second {
		t.Fatal("stalled TLS handshake was not bounded")
	}
	eventually(t, func() bool {
		for _, f := range store.Batch(0, 0).Flows {
			if f.Error != "" {
				return true
			}
		}
		return false
	})
}

func TestAuditHARRejectsInvalidEncodedBody(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	input := `{"log":{"version":"1.2","entries":[{"request":{"method":"GET","url":"http://example.test/"},"response":{"content":{"text":"%%%","encoding":"base64"}}}]}}`
	if _, err = store.ImportHAR(strings.NewReader(input)); err == nil || store.Batch(0, 0).Total != 0 {
		t.Fatal("invalid HAR body was silently accepted")
	}
}

func TestAuditPassthroughTunnelIsVisible(t *testing.T) {
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "passthrough") }))
	defer origin.Close()
	client, store, _ := testProxyOptions(t, false, true, nil)
	roots := x509.NewCertPool()
	roots.AddCert(origin.Certificate())
	client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: roots}
	res, err := client.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)
	eventually(t, func() bool {
		for _, flow := range store.Batch(0, 0).Flows {
			if flow.Method == "CONNECT" && flow.Status == 200 && flow.ContentType == "application/x-connect-tunnel" && strings.HasPrefix(flow.URL, "https://") {
				return true
			}
		}
		return false
	})
}

func TestAuditAbsoluteFormWebSocket(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		kind, data, err := conn.ReadMessage()
		if err == nil {
			conn.WriteMessage(kind, data)
		}
	}))
	defer origin.Close()
	client, store, _ := testProxy(t, true)
	proxyURL, _ := client.Transport.(*http.Transport).Proxy(&http.Request{})
	conn, err := net.Dial("tcp", proxyURL.Host)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	target, _ := url.Parse(origin.URL)
	fmt.Fprintf(conn, "GET %s/ws HTTP/1.1\r\nHost: %s\r\nConnection: keep-alive, Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n", origin.URL, target.Host)
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, &http.Request{Method: "GET"})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 101 {
		t.Fatal(response.StatusCode)
	}
	payload := []byte("echo")
	frame := []byte{0x81, 0x84, 1, 2, 3, 4}
	for i, v := range payload {
		frame = append(frame, v^byte(i+1))
	}
	if _, err = conn.Write(frame); err != nil {
		t.Fatal(err)
	}
	echo := make([]byte, 6)
	if _, err = io.ReadFull(reader, echo); err != nil || !bytes.Equal(echo, append([]byte{0x81, 4}, payload...)) {
		t.Fatalf("absolute-form echo: %v %v", echo, err)
	}
	eventually(t, func() bool {
		for _, flow := range store.Batch(0, 0).Flows {
			if flow.Status == 101 {
				return true
			}
		}
		return false
	})
}

func TestAuditIdleCheckpointSkipsUnchangedSession(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	state := &flowState{detail: Detail{Summary: Summary{ID: "checkpoint", Started: time.Now().UnixMilli()}}}
	store.update(state, true)
	store.Flush()
	path := filepath.Join(dir, "session.json")
	old := time.Unix(946684800, 0)
	if err = os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	store.Flush()
	info, err := os.Stat(path)
	if err != nil || !info.ModTime().Equal(old) {
		t.Fatal("idle checkpoint rewrote session", err)
	}
	store.append(state, false, []byte("new body"))
	store.Flush()
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(data, []byte(`"responseCaptured":8`)) {
		t.Fatal("body change was not checkpointed", err)
	}
}

func TestAuditCheckpointErrorsAreVisible(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(dir, "session.json"), 0700); err != nil {
		t.Fatal(err)
	}
	store.update(&flowState{detail: Detail{Summary: Summary{ID: "write-error"}}}, true)
	store.Flush()
	if store.Error() == "" {
		t.Fatal("checkpoint failure was silently discarded")
	}
}
