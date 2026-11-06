package agent

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/cert"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	"mitmAny/internal/capture"
)

func TestLiveProxyToAgent(t *testing.T) {
	s, dir := fixture(t)
	observer := capture.NewObserver(s.Store)
	defer observer.Close()
	s.Dropped = observer.Dropped
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
	done := make(chan struct{})
	go func() { defer close(done); engine.Serve(listener) }()
	defer func() { engine.Close(); <-done }()
	local, err := Listen(dir, s)
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"items":[{"id":123}],"access_token":"private-token"}`)
	}))
	defer origin.Close()
	o := callOK(t, s, "begin_observation", Args{}).(Observation)
	proxyURL, _ := url.Parse("http://" + listener.Addr().String())
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	res, err := client.Get(origin.URL + "/orders")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cursor := o.Cursor
	for {
		raw, err := s.Call(ctx, "wait_for_flows", Args{ObservationID: o.ID, After: cursor, Timeout: 1000})
		if err != nil {
			t.Fatal(err)
		}
		batch := raw.(map[string]any)
		cursor = batch["cursor"].(uint64)
		for _, flow := range batch["flows"].([]capture.Summary) {
			if !flow.Complete {
				continue
			}
			body, err := RemoteCall(ctx, dir, "read_body", Args{FlowID: flow.ID, ObservationID: o.ID, Direction: "response"})
			if err != nil {
				t.Fatal(err)
			}
			text := body.(map[string]any)["text"].(string)
			if text == "" || body.(map[string]any)["capture_complete"] != true {
				t.Fatal(body)
			}
			return
		}
		if ctx.Err() != nil {
			t.Fatal(ctx.Err())
		}
	}
}
