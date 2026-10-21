package capture

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const MaxEntries = 3000
const MaxDiskBytes int64 = 256 * 1024 * 1024

type removal struct {
	id       string
	revision uint64
}
type Store struct {
	removals     []removal
	removalFloor uint64
	mu           sync.RWMutex
	flushMu      sync.Mutex
	dir          string
	entries      map[string]*Detail
	order        []string
	revision     uint64
	epoch        uint64
	diskBytes    int64
	previewBytes int64
	changed      bool
	generation   uint64
	lastErr      string
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, entries: map[string]*Detail{}, epoch: uint64(time.Now().UnixMilli()) * 1000}
	data, err := os.ReadFile(filepath.Join(dir, "session.json"))
	if err == nil {
		var saved []Detail
		if json.Unmarshal(data, &saved) == nil {
			for _, d := range saved {
				d.Domain, d.RootDomain = DomainNames(d.Host)
				d.RequestBody = ""
				d.ResponseBody = ""
				d.Complete = true
				if d.Status == 0 && d.Error == "" {
					d.Error = "Capture interrupted"
				}
				s.revision++
				d.Revision = s.epoch + s.revision
				item := d
				s.entries[d.ID] = &item
				s.order = append(s.order, d.ID)
				s.diskBytes += d.RequestCaptured + d.ResponseCaptured
				s.previewBytes += d.RequestCaptured
				if !isHTML(d.ContentType) {
					s.previewBytes += d.ResponseCaptured
				}
			}
		}
	}
	// Remove partial body files left by an interrupted process before metadata was saved.
	files, _ := os.ReadDir(dir)
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".body") {
			id := strings.Split(f.Name(), ".")[0]
			if _, ok := s.entries[id]; !ok {
				_ = os.Remove(filepath.Join(dir, f.Name()))
			}
		}
	}
	s.trimLocked()
	return s, nil
}
func (s *Store) bodyPath(id string, request bool) string {
	suffix := ".response.body"
	if request {
		suffix = ".request.body"
	}
	return filepath.Join(s.dir, id+suffix)
}
func (s *Store) update(st *flowState, initial bool) {
	st.mu.Lock()
	d := st.detail
	st.mu.Unlock()
	d.RequestSize = st.requestBytes.Load()
	d.ResponseSize = st.responseBytes.Load()
	d.Truncated = st.lost.Load()
	if !d.Complete {
		d.Duration = float64(time.Now().UnixMilli() - d.Started)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.entries[d.ID]; ok {
		d.Domain, d.RootDomain = old.Domain, old.RootDomain
		d.RequestCaptured = old.RequestCaptured
		d.ResponseCaptured = old.ResponseCaptured
	} else {
		if !initial {
			return
		}
		d.Domain, d.RootDomain = DomainNames(d.Host)
		s.order = append(s.order, d.ID)
	}
	s.revision++
	d.Revision = s.epoch + s.revision
	s.entries[d.ID] = &d
	s.changed = true
	s.generation++
	s.trimLocked()
}
func (s *Store) append(st *flowState, request bool, data []byte) {
	id := st.detail.ID
	// Only the worker calls append/update, so no file I/O is inside the metadata mutex.
	s.mu.RLock()
	_, exists := s.entries[id]
	s.mu.RUnlock()
	if !exists {
		return
	}
	f, err := os.OpenFile(s.bodyPath(id, request), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	var n int
	if err == nil {
		n, err = f.Write(data)
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.lastErr = err.Error()
		st.lost.Store(true)
	}
	if d := s.entries[id]; d != nil {
		if request {
			d.RequestCaptured += int64(n)
		} else {
			d.ResponseCaptured += int64(n)
		}
	}
	s.diskBytes += int64(n)
	if d := s.entries[id]; d != nil && (request || !isHTML(d.ContentType)) {
		s.previewBytes += int64(n)
	}
	if n > 0 {
		s.changed = true
		s.generation++
	}
	s.trimLocked()
}
func (s *Store) trimLocked() {
	// HTML bodies are not subject to the preview byte budget. Count-based
	// session retention still applies, and other bodies retain their byte budget.
	for len(s.order) > MaxEntries || s.previewBytes > MaxDiskBytes {
		id := s.order[0]
		s.order = s.order[1:]
		if d := s.entries[id]; d != nil {
			s.diskBytes -= d.RequestCaptured + d.ResponseCaptured
			s.previewBytes -= d.RequestCaptured
			if !isHTML(d.ContentType) {
				s.previewBytes -= d.ResponseCaptured
			}
		}
		delete(s.entries, id)
		s.revision++
		s.removals = append(s.removals, removal{id, s.epoch + s.revision})
		if len(s.removals) > MaxEntries*2 {
			s.removalFloor = s.removals[0].revision
			s.removals = s.removals[1:]
		}
		_ = os.Remove(s.bodyPath(id, true))
		_ = os.Remove(s.bodyPath(id, false))
		s.changed = true
		s.generation++
	}
}
func (s *Store) Batch(cursor uint64, dropped uint64) Batch {
	s.mu.RLock()
	defer s.mu.RUnlock()
	reset := cursor < s.epoch || cursor > s.epoch+s.revision || cursor < s.removalFloor
	b := Batch{Flows: []Summary{}, Cursor: s.epoch + s.revision, Reset: reset, Total: len(s.entries), Bytes: s.diskBytes, Dropped: dropped}
	if !reset {
		for _, r := range s.removals {
			if r.revision > cursor {
				b.Removed = append(b.Removed, r.id)
			}
		}
	}
	for _, id := range s.order {
		d := s.entries[id]
		if reset || d.Revision > cursor {
			b.Flows = append(b.Flows, d.Summary)
		}
	}
	return b
}
func (s *Store) Detail(id string) (Detail, error) {
	s.mu.RLock()
	d, ok := s.entries[id]
	if !ok {
		s.mu.RUnlock()
		return Detail{}, errors.New("request no longer available")
	}
	copy := *d
	s.mu.RUnlock()
	// IDs come from the indexed store, never directly from a path supplied by the UI.
	req, _ := os.ReadFile(s.bodyPath(copy.ID, true))
	res, _ := os.ReadFile(s.bodyPath(copy.ID, false))
	copy.RequestBody, copy.RequestEncoding = bodyForUI(req, copy.RequestHeaders.Get("Content-Encoding"))
	copy.ResponseBody, copy.ResponseEncoding = bodyForUI(res, copy.ResponseHeaders.Get("Content-Encoding"), isHTML(copy.ContentType))
	return copy, nil
}
func bodyForUI(data []byte, encoding string, unlimitedHTML ...bool) (string, string) {
	unlimited := len(unlimitedHTML) > 0 && unlimitedHTML[0]
	original := data
	codings := strings.Split(strings.ToLower(encoding), ",")
	// Decode in reverse application order. HTML responses have no decoded size cap.
	if len(codings) > 4 {
		return base64.StdEncoding.EncodeToString(original), "compressed"
	}
	for i := len(codings) - 1; i >= 0; i-- {
		coding := strings.TrimSpace(codings[i])
		if coding == "" || coding == "identity" {
			continue
		}
		var r io.Reader
		var closeReader func()
		var err error
		switch coding {
		case "br":
			r = brotli.NewReader(bytes.NewReader(data))
		case "gzip":
			var reader *gzip.Reader
			reader, err = gzip.NewReader(bytes.NewReader(data))
			if err == nil {
				r = reader
				closeReader = func() { _ = reader.Close() }
			}
		case "deflate":
			var reader io.ReadCloser
			reader, err = zlib.NewReader(bytes.NewReader(data))
			if err != nil {
				reader = flate.NewReader(bytes.NewReader(data))
				err = nil
			}
			r = reader
			closeReader = func() { _ = reader.Close() }
		case "zstd":
			var reader *zstd.Decoder
			options := []zstd.DOption{zstd.WithDecoderConcurrency(1), zstd.WithDecoderLowmem(true)}
			if !unlimited {
				options = append(options, zstd.WithDecoderMaxMemory(8<<20), zstd.WithDecoderMaxWindow(8<<20))
			}
			reader, err = zstd.NewReader(bytes.NewReader(data), options...)
			if err == nil {
				r = reader
				closeReader = reader.Close
			}
		default:
			return base64.StdEncoding.EncodeToString(original), "compressed"
		}
		if err != nil {
			return base64.StdEncoding.EncodeToString(original), "compressed"
		}
		source := r
		if !unlimited {
			source = io.LimitReader(r, PreviewLimit+1)
		}
		decoded, readErr := io.ReadAll(source)
		if closeReader != nil {
			closeReader()
		}
		if readErr != nil || (!unlimited && len(decoded) > PreviewLimit) {
			return base64.StdEncoding.EncodeToString(original), "compressed"
		}
		data = decoded
	}
	return base64.StdEncoding.EncodeToString(data), "base64"
}

func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range s.order {
		_ = os.Remove(s.bodyPath(id, true))
		_ = os.Remove(s.bodyPath(id, false))
	}
	s.entries = map[string]*Detail{}
	s.order = nil
	s.diskBytes = 0
	s.previewBytes = 0
	s.epoch = s.epoch + s.revision + 1
	s.revision = 0
	s.removals = nil
	s.removalFloor = 0
	s.changed = true
	s.generation++
}
func (s *Store) flush() {
	s.flushMu.Lock()
	defer s.flushMu.Unlock()
	s.mu.RLock()
	if !s.changed {
		s.mu.RUnlock()
		return
	}
	generation := s.generation
	data := make([]Detail, 0, len(s.order))
	for _, id := range s.order {
		data = append(data, *s.entries[id])
	}
	s.mu.RUnlock()
	encoded, err := json.Marshal(data)
	if err != nil {
		return
	}
	tmp := filepath.Join(s.dir, "session.json.tmp")
	err = os.WriteFile(tmp, encoded, 0600)
	if err == nil {
		err = os.Rename(tmp, filepath.Join(s.dir, "session.json"))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.lastErr = "session checkpoint: " + err.Error()
		return
	}
	if s.generation == generation {
		s.changed = false
	}
}

// Flush checkpoints metadata; body files are written incrementally.
func (s *Store) Flush()        { s.flush() }
func (s *Store) Error() string { s.mu.RLock(); defer s.mu.RUnlock(); return s.lastErr }
