// Package agent exposes bounded, read-only traffic inspection. Observation
// mutations only affect logical bookmarks, never proxy settings or upstreams.
package agent

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"mitmAny/internal/capture"
)

type Args struct {
	ObservationID string `json:"observation_id,omitempty"`
	FlowID        string `json:"flow_id,omitempty"`
	Host          string `json:"host,omitempty"`
	Path          string `json:"path,omitempty"`
	Method        string `json:"method,omitempty"`
	Status        int    `json:"status,omitempty"`
	After         uint64 `json:"after_cursor,omitempty"`
	Before        uint64 `json:"before_cursor,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	Label         string `json:"label,omitempty"`
	Direction     string `json:"direction,omitempty"`
	Offset        int    `json:"offset,omitempty"`
	Timeout       int    `json:"timeout_ms,omitempty"`
}
type Mark struct {
	Label string `json:"label"`
	Time  int64  `json:"time_ms"`
}
type Observation struct {
	ID      string `json:"id"`
	Host    string `json:"host"`
	Started int64  `json:"started_ms"`
	Ended   int64  `json:"ended_ms,omitempty"`
	Cursor  uint64 `json:"start_cursor"`
	Marks   []Mark `json:"marks"`
}
type Service struct {
	Store        *capture.Store
	Status       func() any
	Dropped      func() uint64
	mu           sync.Mutex
	observations map[string]Observation
	salt         []byte
}

func New(store *capture.Store, status func() any, dropped func() uint64) *Service {
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}
	return &Service{Store: store, Status: status, Dropped: dropped, observations: map[string]Observation{}, salt: salt}
}
func (s *Service) batch(cursor uint64) capture.Batch { return s.Store.Batch(cursor, s.Dropped()) }
func (s *Service) token(v string) string {
	mac := hmac.New(sha256.New, s.salt)
	mac.Write([]byte(v))
	return "[redacted:" + hex.EncodeToString(mac.Sum(nil)[:6]) + "]"
}
func sensitive(k string) bool {
	k = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(k, "-", ""), "_", ""))
	for _, v := range []string{"authorization", "cookie", "token", "password", "secret", "apikey", "sessionid", "signature"} {
		if strings.Contains(k, v) {
			return true
		}
	}
	return false
}
func (s *Service) redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "[invalid URL]"
	}
	u.User = nil
	u.Fragment = ""
	q := u.Query()
	for k, vs := range q {
		if sensitive(k) {
			for i, v := range vs {
				vs[i] = s.token(v)
			}
			q[k] = vs
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}
func (s *Service) headers(h http.Header) http.Header {
	for k, vs := range h {
		if sensitive(k) {
			for i, v := range vs {
				vs[i] = s.token(v)
			}
			h[k] = vs
		}
	}
	return h
}
func (s *Service) jsonValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if sensitive(k) {
				b, _ := json.Marshal(val)
				if str, ok := val.(string); ok {
					b = []byte(str)
				}
				x[k] = s.token(string(b))
			} else {
				x[k] = s.jsonValue(val)
			}
		}
	case []any:
		for i, val := range x {
			x[i] = s.jsonValue(val)
		}
	}
	return v
}
func (s *Service) observation(id string) (Observation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.observations[id]
	if !ok {
		return o, errors.New("unknown observation; observations expire on application restart")
	}
	o.Marks = append([]Mark{}, o.Marks...)
	return o, nil
}
func (s *Service) matches(f capture.Summary, a Args, o Observation) bool {
	if o.ID != "" && (f.Started < o.Started || (o.Ended != 0 && f.Started > o.Ended)) {
		return false
	}
	for _, host := range []string{o.Host, a.Host} {
		if host != "" && !strings.EqualFold(f.Host, host) && !strings.EqualFold(f.Domain, host) {
			return false
		}
	}
	return (a.Path == "" || strings.Contains(f.Path, a.Path)) && (a.Method == "" || strings.EqualFold(f.Method, a.Method)) && (a.Status == 0 || f.Status == a.Status) && (a.Before == 0 || f.Revision < a.Before)
}
func (s *Service) flows(a Args, o Observation) (capture.Batch, []capture.Summary) {
	b := s.batch(a.After)
	fs := []capture.Summary{}
	for _, f := range b.Flows {
		if s.matches(f, a, o) {
			f.URL = s.redactURL(f.URL)
			f.Path = s.redactURL(f.Path)
			fs = append(fs, f)
		}
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].Revision > fs[j].Revision })
	return b, fs
}
func (s *Service) Call(ctx context.Context, name string, a Args) (any, error) {
	if a.Limit < 0 || a.Offset < 0 || a.Timeout < 0 || a.Timeout > 30000 {
		return nil, errors.New("invalid bounds: offset/limit >= 0, timeout_ms 0..30000")
	}
	o := Observation{}
	if a.ObservationID != "" {
		var err error
		o, err = s.observation(a.ObservationID)
		if err != nil {
			return nil, err
		}
	}
	switch name {
	case "capture_status":
		b := s.batch(0)
		return map[string]any{"state": s.Status(), "cursor": b.Cursor, "retained_flows": b.Total, "dropped": b.Dropped, "source_attribution": "unknown: desktop and Android share the store", "retention": "shared rolling store; observations do not pin evidence", "body_read_limit_bytes": capture.PreviewLimit, "redaction": "sensitive header/query/JSON/form keys; arbitrary text may contain secrets"}, nil
	case "begin_observation":
		if a.ObservationID != "" {
			return nil, errors.New("begin_observation does not accept an existing observation_id")
		}
		if len(a.Host) > 253 {
			return nil, errors.New("host too long")
		}
		o = Observation{ID: uuid.NewString(), Host: a.Host, Started: time.Now().UnixMilli(), Cursor: s.batch(0).Cursor, Marks: []Mark{}}
		s.mu.Lock()
		defer s.mu.Unlock()
		for id, old := range s.observations {
			if old.Started < time.Now().Add(-24*time.Hour).UnixMilli() {
				delete(s.observations, id)
			}
		}
		if len(s.observations) >= 128 {
			return nil, errors.New("observation capacity reached (128); bookmarks expire after 24 hours")
		}
		s.observations[o.ID] = o
		return o, nil
	case "mark_observation", "end_observation":
		if o.ID == "" {
			return nil, errors.New("observation_id required")
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		o = s.observations[o.ID]
		if name == "end_observation" {
			if o.Ended == 0 {
				o.Ended = time.Now().UnixMilli()
			}
		} else {
			if o.Ended != 0 {
				return nil, errors.New("observation ended")
			}
			if a.Label == "" || len(a.Label) > 200 || len(o.Marks) >= 100 {
				return nil, errors.New("label required, max 200 bytes; max 100 marks")
			}
			o.Marks = append(o.Marks, Mark{a.Label, time.Now().UnixMilli()})
		}
		s.observations[o.ID] = o
		return o, nil
	case "search_flows", "summarize_traffic", "wait_for_flows":
		if name == "wait_for_flows" && a.After == 0 {
			return nil, errors.New("after_cursor required; use begin_observation.start_cursor")
		}
		deadline := time.Now().Add(time.Duration(a.Timeout) * time.Millisecond)
		for {
			if a.ObservationID != "" {
				var err error
				o, err = s.observation(a.ObservationID)
				if err != nil {
					return nil, err
				}
			}
			b, fs := s.flows(a, o)
			if name != "wait_for_flows" || len(fs) > 0 || b.Reset || time.Now().After(deadline) {
				if name == "summarize_traffic" {
					type Group struct {
						Host   string `json:"host"`
						Method string `json:"method"`
						Path   string `json:"path"`
						Status int    `json:"status"`
						Count  int    `json:"count"`
						FlowID string `json:"example_flow_id"`
					}
					groups := map[string]*Group{}
					for _, f := range fs {
						if u, err := url.Parse(f.Path); err == nil {
							f.Path = u.EscapedPath()
						}
						key := fmt.Sprintf("%s\x00%s\x00%s\x00%d", f.Host, f.Method, f.Path, f.Status)
						g := groups[key]
						if g == nil {
							g = &Group{Host: f.Host, Method: f.Method, Path: f.Path, Status: f.Status, FlowID: f.ID}
							groups[key] = g
						}
						g.Count++
					}
					out := []Group{}
					for _, g := range groups {
						out = append(out, *g)
					}
					sort.Slice(out, func(i, j int) bool {
						if out[i].Count != out[j].Count {
							return out[i].Count > out[j].Count
						}
						return fmt.Sprint(out[i]) < fmt.Sprint(out[j])
					})
					limit := a.Limit
					if limit == 0 {
						limit = 30
					}
					if limit > 100 {
						limit = 100
					}
					total := len(out)
					if len(out) > limit {
						out = out[:limit]
					}
					return map[string]any{"groups": out, "total_groups": total, "matched_flows": len(fs), "cursor": b.Cursor, "dropped": b.Dropped, "reset": b.Reset, "removed": b.Removed}, nil
				}
				limit := a.Limit
				if limit == 0 {
					limit = 30
				}
				if limit > 100 {
					limit = 100
				}
				more := len(fs) > limit
				if more {
					fs = fs[:limit]
				}
				var next uint64
				if more {
					next = fs[len(fs)-1].Revision
				}
				return map[string]any{"flows": fs, "cursor": b.Cursor, "next_before_cursor": next, "has_more": more, "reset": b.Reset, "removed": b.Removed, "dropped": b.Dropped, "observation": o}, nil
			}
			timer := time.NewTimer(200 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	case "get_flow", "read_body":
		d, err := s.Store.Metadata(a.FlowID)
		if err != nil {
			return nil, err
		}
		if !s.matches(d.Summary, a, o) {
			return nil, errors.New("flow outside requested scope")
		}
		if name == "get_flow" {
			d.URL = s.redactURL(d.URL)
			d.Path = s.redactURL(d.Path)
			d.RequestHeaders = s.headers(d.RequestHeaders)
			d.ResponseHeaders = s.headers(d.ResponseHeaders)
			d.RequestTrailers = s.headers(d.RequestTrailers)
			d.ResponseTrailers = s.headers(d.ResponseTrailers)
			return d, nil
		}
		if a.Direction != "request" && a.Direction != "response" {
			return nil, errors.New("direction must be request or response")
		}
		req := a.Direction == "request"
		data, encoding, limited, err := s.Store.AgentBody(a.FlowID, req)
		if err != nil {
			return nil, err
		}
		if encoding == "compressed" || !utf8.Valid(data) {
			return map[string]any{"flow_id": a.FlowID, "readable": false, "reason": "binary, incomplete compressed body, unsupported encoding or decode limit", "captured_bytes": len(data)}, nil
		}
		ct := d.ResponseHeaders.Get("Content-Type")
		if ct == "" {
			ct = d.ContentType
		}
		if req {
			ct = d.RequestHeaders.Get("Content-Type")
		}
		if strings.Contains(strings.ToLower(ct), "json") && !json.Valid(data) && len(data) > 0 {
			return map[string]any{"flow_id": a.FlowID, "readable": false, "reason": "incomplete or invalid JSON; withheld because key-based redaction cannot be applied safely", "capture_truncated": d.Truncated, "read_limit_reached": limited}, nil
		}
		var value any
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.UseNumber()
		if json.Valid(data) && decoder.Decode(&value) == nil {
			data, _ = json.Marshal(s.jsonValue(value))
		} else {
			ct := d.ResponseHeaders.Get("Content-Type")
			if req {
				ct = d.RequestHeaders.Get("Content-Type")
			}
			if strings.Contains(ct, "application/x-www-form-urlencoded") {
				q, e := url.ParseQuery(string(data))
				if e == nil {
					for k, vs := range q {
						if sensitive(k) {
							for i, v := range vs {
								vs[i] = s.token(v)
							}
							q[k] = vs
						}
					}
					data = []byte(q.Encode())
				}
			}
		}
		captured, observed := d.ResponseCaptured, d.ResponseSize
		if req {
			captured, observed = d.RequestCaptured, d.RequestSize
		}
		complete := d.Complete && !d.Truncated && !limited && d.Error == "" && captured == observed && d.ContentType != "application/x-connect-tunnel"
		// Paging uses Unicode code points in the redacted representation.
		chars := []rune(string(data))
		if a.Offset > len(chars) {
			return nil, errors.New("offset exceeds redacted body length")
		}
		limit := a.Limit
		if limit == 0 {
			limit = 8000
		}
		if limit > 32000 {
			limit = 32000
		}
		end := a.Offset + limit
		if end > len(chars) {
			end = len(chars)
		}
		return map[string]any{"flow_id": a.FlowID, "text": string(chars[a.Offset:end]), "offset": a.Offset, "next_offset": end, "has_more": end < len(chars), "readable": true, "capture_complete": complete, "captured_bytes": captured, "observed_bytes": observed, "flow_complete": d.Complete, "capture_truncated": d.Truncated, "read_limit_reached": limited, "redaction": "known sensitive keys only; inspect scope before sharing"}, nil
	default:
		return nil, errors.New("unknown tool")
	}
}
