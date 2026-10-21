package capture

import "net/http"

type Summary struct {
	ID           string  `json:"id"`
	Method       string  `json:"method"`
	URL          string  `json:"url"`
	Host         string  `json:"host"`
	Path         string  `json:"path"`
	Scheme       string  `json:"scheme"`
	Protocol     string  `json:"protocol"`
	Status       int     `json:"status"`
	ContentType  string  `json:"contentType"`
	Started      int64   `json:"started"`
	Duration     float64 `json:"duration"`
	TTFB         float64 `json:"ttfb"`
	RequestSize  int64   `json:"requestSize"`
	ResponseSize int64   `json:"responseSize"`
	Complete     bool    `json:"complete"`
	Error        string  `json:"error,omitempty"`
	Truncated    bool    `json:"truncated"`
	Revision     uint64  `json:"revision"`
}
type Detail struct {
	Summary
	RequestTrailers  http.Header `json:"requestTrailers,omitempty"`
	ResponseTrailers http.Header `json:"responseTrailers,omitempty"`
	RequestHeaders   http.Header `json:"requestHeaders"`
	ResponseHeaders  http.Header `json:"responseHeaders"`
	RequestBody      string      `json:"requestBody"`
	ResponseBody     string      `json:"responseBody"`
	RequestEncoding  string      `json:"requestEncoding"`
	ResponseEncoding string      `json:"responseEncoding"`
	RequestCaptured  int64       `json:"requestCaptured"`
	ResponseCaptured int64       `json:"responseCaptured"`
}
type Batch struct {
	Removed []string  `json:"removed"`
	Flows   []Summary `json:"flows"`
	Cursor  uint64    `json:"cursor"`
	Reset   bool      `json:"reset"`
	Total   int       `json:"total"`
	Bytes   int64     `json:"bytes"`
	Dropped uint64    `json:"dropped"`
}
