package proxy

import (
	"context"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
)

// Reuse forwarding buffers without making network forwarding wait for capture buffers.
var forwardingBuffers = sync.Pool{New: func() any { b := make([]byte, 32*1024); return &b }}

type upgradeBufferPool struct{}

func (upgradeBufferPool) Get() []byte  { return *forwardingBuffers.Get().(*[]byte) }
func (upgradeBufferPool) Put(b []byte) { forwardingBuffers.Put(&b) }

func headerHasToken(value, token string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}
func isWebSocketUpgrade(req *http.Request) bool {
	return headerHasToken(req.Header.Get("Connection"), "upgrade") && strings.EqualFold(req.Header.Get("Upgrade"), "websocket")
}
func removeHopHeaders(header http.Header) {
	for _, value := range header.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			header.Del(strings.TrimSpace(name))
		}
	}
	for _, name := range []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade"} {
		header.Del(name)
	}
}

// Use the standard library's bidirectional upgrade relay in capture-only mode.
// This preserves handshake headers, subprotocols, control frames and streaming,
// while recording a single flow for the lifetime of the WebSocket connection.
func (a *attacker) serveWebSocket(res http.ResponseWriter, req *http.Request, transport http.RoundTripper) {
	f := newFlow()
	f.Request = newRequest(req)
	f.ConnContext = req.Context().Value(connContextKey).(*ConnContext)
	f.ConnContext.FlowCount.Add(1)
	defer f.finish()
	for _, addon := range a.proxy.Addons {
		addon.Requestheaders(f)
	}
	if transport == nil {
		transport = a.client.Transport
	}
	relay := &httputil.ReverseProxy{
		Rewrite:    func(pr *httputil.ProxyRequest) { pr.Out.Host = pr.In.Host },
		Transport:  transport,
		BufferPool: upgradeBufferPool{},
		ModifyResponse: func(response *http.Response) error {
			f.Response = &Response{StatusCode: response.StatusCode, Header: response.Header.Clone(), Trailer: response.Trailer}
			for _, addon := range a.proxy.Addons {
				addon.Responseheaders(f)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			for _, addon := range a.proxy.Addons {
				addon.RequestError(f, err)
			}
			w.WriteHeader(http.StatusBadGateway)
		},
	}
	ctx := context.WithValue(req.Context(), proxyReqCtxKey, req)
	relay.ServeHTTP(res, req.WithContext(ctx))
}
