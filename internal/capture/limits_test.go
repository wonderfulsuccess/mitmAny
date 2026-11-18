package capture

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestHTMLCaptureLimitPreservesForwarding(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), HTMLLimit+chunkSize)
	o := &Observer{queue: make(chan event, 1), blocks: make(chan []byte, 1)}
	o.blocks <- make([]byte, chunkSize)
	st := &flowState{}
	tap := &tapReader{Reader: bytes.NewReader(payload), observer: o, state: st, limit: HTMLLimit}
	var forwarded, captured bytes.Buffer
	buf := make([]byte, chunkSize)
	for {
		n, err := tap.Read(buf)
		forwarded.Write(buf[:n])
		select {
		case e := <-o.queue:
			captured.Write(e.data)
			o.blocks <- e.data[:chunkSize]
		default:
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(forwarded.Bytes(), payload) || captured.Len() != HTMLLimit || !st.lost.Load() {
		t.Fatal("limit changed forwarding or failed to truncate capture")
	}
	if st.detail.CaptureNote != "Capture size limit reached" {
		t.Fatal(st.detail.CaptureNote)
	}
}

func TestHTMLSaturatedCaptureDoesNotWait(t *testing.T) {
	// With no blocks and no worker, a blocking implementation would hang this test.
	o := &Observer{queue: make(chan event, 1), blocks: make(chan []byte, 1)}
	st := &flowState{}
	tap := &tapReader{Reader: strings.NewReader("still forwarded"), observer: o, state: st, limit: HTMLLimit}
	data, err := io.ReadAll(tap)
	if err != nil || string(data) != "still forwarded" || !st.lost.Load() || o.Dropped() != 1 {
		t.Fatal("saturated HTML capture did not fail open")
	}
}

func TestHTMLDecodedLimit(t *testing.T) {
	var zipped bytes.Buffer
	zw := gzip.NewWriter(&zipped)
	zw.Write(bytes.Repeat([]byte("x"), HTMLLimit+1))
	zw.Close()
	encoded, encoding := bodyForUI(zipped.Bytes(), "gzip", true)
	raw, _ := base64.StdEncoding.DecodeString(encoded)
	if encoding != "compressed" || !bytes.Equal(raw, zipped.Bytes()) {
		t.Fatal("oversized decompression should preserve compressed bytes")
	}
}

func TestHARHTMLLimit(t *testing.T) {
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	html := strings.Repeat("x", HTMLLimit+1)
	doc := fmt.Sprintf(`{"log":{"version":"1.2","entries":[{"request":{"method":"GET","url":"https://example.test/"},"response":{"content":{"mimeType":"text/html","text":%q}}}]}}`, html)
	if _, err := s.ImportHAR(strings.NewReader(doc)); err != nil {
		t.Fatal(err)
	}
	d, err := s.Detail(s.Batch(0, 0).Flows[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Truncated || d.CaptureNote == "" || d.ResponseCaptured != HTMLLimit || s.diskBytes != HTMLLimit {
		t.Fatal("HAR bypassed limits")
	}
	// Legacy oversized on-disk bodies must also be bounded on read.
	if err := os.WriteFile(s.bodyPath(d.ID, false), []byte(html), 0600); err != nil {
		t.Fatal(err)
	}
	d, err = s.Detail(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(d.ResponseBody)
	if len(raw) != HTMLLimit || !d.Truncated {
		t.Fatal("legacy read bypassed limit")
	}
}
