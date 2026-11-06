package capture

import (
	"encoding/base64"
	"errors"
	"io"
	"os"
)

// Metadata never reads body files. Header maps are cloned for independent consumers.
func (s *Store) Metadata(id string) (Detail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := s.entries[id]
	if d == nil {
		return Detail{}, errors.New("flow unavailable: evicted or cleared")
	}
	c := *d
	c.RequestHeaders = d.RequestHeaders.Clone()
	c.ResponseHeaders = d.ResponseHeaders.Clone()
	c.RequestTrailers = d.RequestTrailers.Clone()
	c.ResponseTrailers = d.ResponseTrailers.Clone()
	return c, nil
}

// AgentBody has bounded input and decoding, including HTML. It never silently
// interprets a truncated compressed stream as successfully decoded content.
func (s *Store) AgentBody(id string, request bool) ([]byte, string, bool, error) {
	d, err := s.Metadata(id)
	if err != nil {
		return nil, "", false, err
	}
	f, err := os.Open(s.bodyPath(d.ID, request))
	if err != nil {
		size := d.ResponseCaptured
		if request {
			size = d.RequestCaptured
		}
		if os.IsNotExist(err) && size == 0 {
			return []byte{}, "base64", false, nil
		}
		return nil, "", false, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, PreviewLimit+1))
	if err != nil {
		return nil, "", false, err
	}
	limited := len(raw) > PreviewLimit
	if limited {
		raw = raw[:PreviewLimit]
	}
	headers := d.ResponseHeaders
	if request {
		headers = d.RequestHeaders
	}
	encoded, encoding := bodyForUI(raw, headers.Get("Content-Encoding"))
	data, err := base64.StdEncoding.DecodeString(encoded)
	return data, encoding, limited, err
}
