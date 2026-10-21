package capture

import (
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

const PreviewLimit = 1024 * 1024
const chunkSize = 32 * 1024

func isHTML(contentType string) bool {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	return strings.EqualFold(mediaType, "text/html") || strings.EqualFold(mediaType, "application/xhtml+xml")
}

type flowState struct {
	mu            sync.Mutex
	detail        Detail
	requestBytes  atomic.Int64
	responseBytes atomic.Int64
	lost          atomic.Bool
}
type event struct {
	work    func()
	state   *flowState
	kind    byte
	data    []byte
	request bool
	ack     chan struct{}
}

// Ordinary captures copy a bounded prefix without waiting for a queue slot.
// HTML captures retain the full response, applying bounded queue backpressure.
type Observer struct {
	proxy.BaseAddon
	store     *Store
	queue     chan event
	blocks    chan []byte
	states    sync.Map
	slots     chan struct{}
	recording atomic.Bool
	dropped   atomic.Uint64
	stop      chan struct{}
	done      chan struct{}
	once      sync.Once
}

func NewObserver(store *Store) *Observer {
	o := &Observer{store: store, queue: make(chan event, 512), blocks: make(chan []byte, 256), slots: make(chan struct{}, 1024), stop: make(chan struct{}), done: make(chan struct{})}
	for i := 0; i < 256; i++ {
		o.blocks <- make([]byte, chunkSize)
	}
	o.recording.Store(true)
	go o.run()
	return o
}
func (o *Observer) SetRecording(v bool) { o.recording.Store(v) }
func (o *Observer) Dropped() uint64     { return o.dropped.Load() }
func (o *Observer) Close()              { o.once.Do(func() { close(o.stop) }); <-o.done }
func (o *Observer) Requestheaders(f *proxy.Flow) {
	f.Stream = true
	// Plain HTTP can use the shared transport's connection pool and standard
	// explicit upstream-proxy handling, without opening CONNECT-to-port-80 tunnels.
	f.UseSeparateClient = f.Request.URL.Scheme == "http"
	if !o.recording.Load() || f.Request.Method == http.MethodConnect && (f.ConnContext == nil || f.ConnContext.Intercept) {
		return
	}
	select {
	case o.slots <- struct{}{}:
	default:
		o.dropped.Add(1)
		return
	}
	urlCopy := *f.Request.URL
	u := &urlCopy
	contentType := ""
	if f.Request.Method == http.MethodConnect {
		u.Scheme = "https"
		contentType = "application/x-connect-tunnel"
	}
	st := &flowState{detail: Detail{Summary: Summary{ID: f.Id.String(), Method: f.Request.Method, URL: u.String(), Host: u.Host, Path: u.RequestURI(), Scheme: u.Scheme, ContentType: contentType, Protocol: f.Request.Proto, Started: f.StartTime.UnixMilli()}, RequestHeaders: f.Request.Header.Clone(), ResponseHeaders: make(http.Header)}}
	select {
	case o.queue <- event{state: st, kind: 'n'}:
	default:
		<-o.slots
		o.dropped.Add(1)
		return
	}
	o.states.Store(f.Id, st)
	go func() {
		defer func() { o.states.Delete(f.Id); <-o.slots }()
		select {
		case <-f.Done():
		case <-o.stop:
			return
		}
		st.mu.Lock()
		st.detail.Complete = true
		st.detail.Duration = float64(time.Since(f.StartTime).Microseconds()) / 1000
		st.mu.Unlock()
		// Waiting here cannot hold up network forwarding: Flow.Done has already closed.
		select {
		case o.queue <- event{state: st, kind: 'f'}:
		case <-o.stop:
		}
	}()
}
func (o *Observer) Responseheaders(f *proxy.Flow) {
	value, ok := o.states.Load(f.Id)
	if !ok {
		return
	}
	st := value.(*flowState)
	st.mu.Lock()
	st.detail.Status = f.Response.StatusCode
	if contentType := f.Response.Header.Get("Content-Type"); contentType != "" {
		st.detail.ContentType = contentType
	}
	st.detail.ResponseHeaders = f.Response.Header.Clone()
	st.detail.TTFB = float64(time.Since(f.StartTime).Microseconds()) / 1000
	st.mu.Unlock()
	select {
	case o.queue <- event{state: st, kind: 'h'}:
	default:
	}
}
func (o *Observer) RequestError(f *proxy.Flow, err error) {
	if value, ok := o.states.Load(f.Id); ok {
		st := value.(*flowState)
		st.mu.Lock()
		st.detail.Error = err.Error()
		st.mu.Unlock()
	}
}
func (o *Observer) StreamRequestModifier(f *proxy.Flow, r io.Reader) io.Reader {
	return o.wrap(f, r, true)
}
func (o *Observer) StreamResponseModifier(f *proxy.Flow, r io.Reader) io.Reader {
	return o.wrap(f, r, false)
}
func (o *Observer) wrap(f *proxy.Flow, r io.Reader, request bool) io.Reader {
	if r == nil {
		return r
	}
	value, ok := o.states.Load(f.Id)
	if !ok {
		return r
	}
	tap := &tapReader{Reader: r, observer: o, state: value.(*flowState), request: request}
	if !request && f.Response != nil {
		tap.unlimited = isHTML(f.Response.Header.Get("Content-Type"))
	}
	if request && f.Request.Raw() != nil {
		tap.trailers = func() http.Header { return f.Request.Raw().Trailer }
	}
	if !request && f.Response != nil {
		tap.trailers = func() http.Header { return f.Response.Trailer }
	}
	return tap
}

type tapReader struct {
	io.Reader
	observer       *Observer
	state          *flowState
	request        bool
	seen           int64
	previewStopped bool
	unlimited      bool
	trailers       func() http.Header
}

func (t *tapReader) Read(p []byte) (int, error) {
	n, err := t.Reader.Read(p)
	if err == io.EOF && t.trailers != nil {
		trailers := t.trailers().Clone()
		t.state.mu.Lock()
		if t.request {
			t.state.detail.RequestTrailers = trailers
		} else {
			t.state.detail.ResponseTrailers = trailers
		}
		t.state.mu.Unlock()
		t.trailers = nil
	}
	if n == 0 {
		return n, err
	}
	if t.request {
		t.state.requestBytes.Add(int64(n))
	} else {
		t.state.responseBytes.Add(int64(n))
	}
	if t.previewStopped {
		return n, err
	}
	remaining := int64(PreviewLimit) - t.seen
	if t.unlimited {
		remaining = int64(n)
	}
	t.seen += int64(n)
	if remaining <= 0 {
		t.previewStopped = true
		t.state.lost.Store(true)
		return n, err
	}
	count := n
	if int64(count) > remaining {
		count = int(remaining)
		t.state.lost.Store(true)
	}
	for offset := 0; offset < count; {
		var block []byte
		if t.unlimited {
			// Apply bounded backpressure for HTML instead of dropping body chunks.
			select {
			case block = <-t.observer.blocks:
			case <-t.observer.stop:
				t.state.lost.Store(true)
				return n, err
			}
		} else {
			select {
			case block = <-t.observer.blocks:
			default:
				t.previewStopped = true
				t.state.lost.Store(true)
				t.observer.dropped.Add(1)
				return n, err
			}
		}
		size := min(count-offset, len(block))
		copy(block, p[offset:offset+size])
		if t.unlimited {
			select {
			case t.observer.queue <- event{state: t.state, kind: 'b', data: block[:size], request: t.request}:
			case <-t.observer.stop:
				t.observer.blocks <- block[:chunkSize]
				t.state.lost.Store(true)
				return n, err
			}
		} else {
			select {
			case t.observer.queue <- event{state: t.state, kind: 'b', data: block[:size], request: t.request}:
			default:
				t.observer.blocks <- block[:chunkSize]
				t.previewStopped = true
				t.state.lost.Store(true)
				t.observer.dropped.Add(1)
				return n, err
			}
		}
		offset += size
	}
	return n, err
}
func (o *Observer) run() {
	defer close(o.done)
	ticker := time.NewTicker(200 * time.Millisecond)
	checkpoint := time.NewTicker(5 * time.Second)
	defer checkpoint.Stop()
	defer ticker.Stop()
	handle := func(e event) {
		switch e.kind {
		case 'i':
			e.work()
		case 'c':
			o.store.Clear()
			o.store.flush()
			close(e.ack)
		case 'n', 'h', 'f':
			o.store.update(e.state, e.kind == 'n')
		case 'b':
			o.store.append(e.state, e.request, e.data)
			o.blocks <- e.data[:chunkSize]
		}
	}
	for {
		select {
		case e := <-o.queue:
			handle(e)
		case <-checkpoint.C:
			o.store.flush()
		case <-ticker.C:
			o.states.Range(func(_, value any) bool { o.store.update(value.(*flowState), false); return true })
		case <-o.stop:
			for {
				select {
				case e := <-o.queue:
					handle(e)
				default:
					o.store.flush()
					return
				}
			}
		}
	}
}

// CONNECT errors precede HTTP requests; preserve a diagnostic entry for them.
func (o *Observer) HTTPConnectError(f *proxy.Flow, err error) {
	if _, exists := o.states.Load(f.Id); exists {
		o.RequestError(f, err)
		return
	}
	urlCopy := *f.Request.URL
	u := &urlCopy
	u.Scheme = "https"
	st := &flowState{detail: Detail{Summary: Summary{ID: f.Id.String(), Method: "CONNECT", URL: u.String(), Host: u.Host, Path: u.String(), Scheme: "https", Protocol: f.Request.Proto, Started: f.StartTime.UnixMilli(), Duration: float64(time.Since(f.StartTime).Microseconds()) / 1000, Error: err.Error(), Complete: true}, RequestHeaders: f.Request.Header.Clone(), ResponseHeaders: make(http.Header)}}
	if !o.recording.Load() {
		return
	}
	select {
	case o.queue <- event{state: st, kind: 'n'}:
	default:
		o.dropped.Add(1)
	}
}

func (o *Observer) Clear() {
	ack := make(chan struct{})
	select {
	case o.queue <- event{kind: 'c', ack: ack}:
	case <-o.done:
		return
	}
	select {
	case <-ack:
	case <-o.done:
	}
}

// Imports run on the storage worker, sharing its eviction and file ordering.
func (o *Observer) ImportHAR(r io.Reader) (int, error) {
	type result struct {
		count int
		err   error
	}
	done := make(chan result, 1)
	work := func() { n, err := o.store.ImportHAR(r); done <- result{n, err} }
	select {
	case o.queue <- event{kind: 'i', work: work}:
	case <-o.done:
		return 0, io.ErrClosedPipe
	}
	select {
	case out := <-done:
		return out.count, out.err
	case <-o.done:
		return 0, io.ErrClosedPipe
	}
}
