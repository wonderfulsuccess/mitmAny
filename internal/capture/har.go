package capture

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

type harHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type harEntry struct {
	Started string  `json:"startedDateTime"`
	Time    float64 `json:"time"`
	Request struct {
		Method   string      `json:"method"`
		URL      string      `json:"url"`
		Protocol string      `json:"httpVersion"`
		Headers  []harHeader `json:"headers"`
		BodySize int64       `json:"bodySize"`
		PostData struct {
			Text     string `json:"text"`
			MimeType string `json:"mimeType"`
		} `json:"postData"`
	} `json:"request"`
	Response struct {
		Status   int         `json:"status"`
		Headers  []harHeader `json:"headers"`
		BodySize int64       `json:"bodySize"`
		Content  struct {
			Text     string `json:"text"`
			Encoding string `json:"encoding"`
			Size     int64  `json:"size"`
			MimeType string `json:"mimeType"`
		} `json:"content"`
	} `json:"response"`
	Timings struct {
		Wait float64 `json:"wait"`
	} `json:"timings"`
	Meta struct {
		RequestBody      string      `json:"requestBodyBase64"`
		RequestEncoding  string      `json:"requestEncoding"`
		ResponseEncoding string      `json:"responseEncoding"`
		RequestTrailers  http.Header `json:"requestTrailers"`
		ResponseTrailers http.Header `json:"responseTrailers"`
		Truncated        bool        `json:"truncated"`
		Error            string      `json:"error"`
	} `json:"_mitmAny"`
}

func (s *Store) ImportHAR(reader io.Reader) (int, error) {
	var document struct {
		Log struct {
			Version string     `json:"version"`
			Entries []harEntry `json:"entries"`
		} `json:"log"`
	}
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&document); err != nil {
		return 0, fmt.Errorf("无法解析 HAR: %w", err)
	}
	if document.Log.Version == "" {
		return 0, fmt.Errorf("不是有效的 HTTP Archive 文件")
	}
	if len(document.Log.Entries) > MaxEntries {
		return 0, fmt.Errorf("单次最多导入 %d 条请求", MaxEntries)
	}
	// Validate every URL before changing the current session.
	for _, e := range document.Log.Entries {
		u, err := url.Parse(e.Request.URL)
		if err != nil || u == nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return 0, fmt.Errorf("HAR 包含无效的 HTTP URL")
		}
		for _, body := range []struct {
			text    string
			encoded bool
		}{{e.Meta.RequestBody, e.Meta.RequestBody != ""}, {e.Response.Content.Text, e.Response.Content.Encoding == "base64"}} {
			if body.encoded {
				if _, err := base64.StdEncoding.DecodeString(body.text); err != nil {
					return 0, fmt.Errorf("HAR 包含无效的 Base64 正文: %w", err)
				}
			}
		}
	}
	headers := func(h []harHeader) http.Header {
		out := make(http.Header)
		for _, v := range h {
			out.Add(v.Name, v.Value)
		}
		return out
	}
	for _, e := range document.Log.Entries {
		u, _ := url.Parse(e.Request.URL)
		started, err := time.Parse(time.RFC3339Nano, e.Started)
		if err != nil {
			started = time.Now()
		}
		req := []byte(e.Request.PostData.Text)
		if e.Meta.RequestBody != "" {
			req, _ = base64.StdEncoding.DecodeString(e.Meta.RequestBody)
		}
		res := []byte(e.Response.Content.Text)
		if e.Response.Content.Encoding == "base64" {
			res, err = base64.StdEncoding.DecodeString(e.Response.Content.Text)
			if err != nil {
				res = nil
			}
		}
		contentType := e.Response.Content.MimeType
		if contentType == "" {
			contentType = headers(e.Response.Headers).Get("Content-Type")
		}
		html := isHTML(contentType)
		truncated := e.Meta.Truncated || len(req) > PreviewLimit || (!html && len(res) > PreviewLimit)
		req = req[:min(len(req), PreviewLimit)]
		if !html {
			res = res[:min(len(res), PreviewLimit)]
		}
		requestHeaders, responseHeaders := headers(e.Request.Headers), headers(e.Response.Headers)
		// HAR content is decoded per the format. Don't decompress it a second time.
		if e.Meta.ResponseEncoding != "compressed" {
			responseHeaders.Del("Content-Encoding")
		}
		if e.Meta.RequestEncoding != "compressed" {
			requestHeaders.Del("Content-Encoding")
		}
		if responseHeaders.Get("Content-Type") == "" {
			responseHeaders.Set("Content-Type", e.Response.Content.MimeType)
		}
		st := &flowState{detail: Detail{Summary: Summary{ID: uuid.NewString(), Method: e.Request.Method, URL: u.String(), Host: u.Host, Path: u.RequestURI(), Scheme: u.Scheme, Protocol: e.Request.Protocol, Status: e.Response.Status, ContentType: contentType, Started: started.UnixMilli(), Duration: max(0, e.Time), TTFB: max(0, e.Timings.Wait), Complete: true, Error: e.Meta.Error}, RequestHeaders: requestHeaders, ResponseHeaders: responseHeaders, RequestTrailers: e.Meta.RequestTrailers, ResponseTrailers: e.Meta.ResponseTrailers}}
		st.requestBytes.Store(max(int64(len(req)), e.Request.BodySize))
		st.responseBytes.Store(max(int64(len(res)), e.Response.Content.Size))
		st.lost.Store(truncated)
		s.update(st, true)
		s.append(st, true, req)
		s.append(st, false, res)
	}
	s.Flush()
	return len(document.Log.Entries), nil
}
