package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"mitmAny/internal/capture"
)

func fixture(t *testing.T) (*Service, string) {
	t.Helper()
	dir := t.TempDir()
	store, e := capture.NewStore(filepath.Join(dir, "session"))
	if e != nil {
		t.Fatal(e)
	}
	return New(store, func() any { return map[string]any{"running": true} }, func() uint64 { return 0 }), dir
}
func addFlow(t *testing.T, s *Service, host, body string) string {
	t.Helper()
	content, _ := json.Marshal(body)
	har := fmt.Sprintf(`{"log":{"version":"1.2","entries":[{"request":{"method":"GET","url":"https://%s/orders?access_token=querysecret","headers":[{"name":"Authorization","value":"Bearer headersecret"}]},"response":{"status":200,"content":{"mimeType":"application/json","text":%s}}}]}}`, host, content)
	if _, e := s.Store.ImportHAR(strings.NewReader(har)); e != nil {
		t.Fatal(e)
	}
	b := s.batch(0)
	return b.Flows[len(b.Flows)-1].ID
}
func callOK(t *testing.T, s *Service, name string, a Args) any {
	t.Helper()
	r, e := s.Call(context.Background(), name, a)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestObservationFilteringPaginationRedaction(t *testing.T) {
	s, _ := fixture(t)
	o := callOK(t, s, "begin_observation", Args{Host: "api.example.test"}).(Observation)
	id := addFlow(t, s, "api.example.test", `{"access_token":"bodysecret","id":9007199254740993,"text":"你好世界"}`)
	addFlow(t, s, "other.example.test", `{}`)
	addFlow(t, s, "api.example.test", `{}`)
	r := callOK(t, s, "search_flows", Args{ObservationID: o.ID, After: o.Cursor, Limit: 1}).(map[string]any)
	if !r["has_more"].(bool) || len(r["flows"].([]capture.Summary)) != 1 {
		t.Fatal(r)
	}
	r = callOK(t, s, "search_flows", Args{ObservationID: o.ID, After: o.Cursor, Before: r["next_before_cursor"].(uint64), Limit: 1}).(map[string]any)
	if r["has_more"].(bool) || r["flows"].([]capture.Summary)[0].ID != id {
		t.Fatal(r)
	}
	d := callOK(t, s, "get_flow", Args{FlowID: id}).(capture.Detail)
	b, _ := json.Marshal(d)
	if strings.Contains(string(b), "querysecret") || strings.Contains(string(b), "headersecret") {
		t.Fatal("secret leaked")
	}
	original, _ := s.Store.Metadata(id)
	if original.RequestHeaders.Get("Authorization") != "Bearer headersecret" {
		t.Fatal("store mutated")
	}
	body := callOK(t, s, "read_body", Args{FlowID: id, Direction: "response"}).(map[string]any)["text"].(string)
	if strings.Contains(body, "bodysecret") || !strings.Contains(body, "9007199254740993") {
		t.Fatal(body)
	}
	page := callOK(t, s, "read_body", Args{FlowID: id, Direction: "response", Offset: 2, Limit: 3}).(map[string]any)["text"].(string)
	if page != string([]rune(body)[2:5]) {
		t.Fatal(page)
	}
	callOK(t, s, "mark_observation", Args{ObservationID: o.ID, Label: "login"})
	callOK(t, s, "end_observation", Args{ObservationID: o.ID})
	if _, e := s.Call(context.Background(), "mark_observation", Args{ObservationID: o.ID, Label: "late"}); e == nil {
		t.Fatal("ended mark accepted")
	}
}
func TestWaitResetAndIncompleteJSON(t *testing.T) {
	s, _ := fixture(t)
	cursor := s.batch(0).Cursor
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := s.Call(ctx, "wait_for_flows", Args{After: cursor, Timeout: 30000}); e == nil {
		t.Fatal("wait ignored cancellation")
	}
	s.Store.Clear()
	r := callOK(t, s, "wait_for_flows", Args{After: cursor, Timeout: 30000}).(map[string]any)
	if !r["reset"].(bool) {
		t.Fatal(r)
	}
	id := addFlow(t, s, "api.test", `{"token":"never-leak`)
	body := callOK(t, s, "read_body", Args{FlowID: id, Direction: "response"}).(map[string]any)
	if body["readable"] != false {
		t.Fatal(body)
	}
	s.Store.Clear()
	if _, e := s.Call(context.Background(), "get_flow", Args{FlowID: id}); e == nil {
		t.Fatal("cleared flow readable")
	}
}
func TestAuthenticatedTransportAndMCP(t *testing.T) {
	s, dir := fixture(t)
	local, e := Listen(dir, s)
	if e != nil {
		t.Fatal(e)
	}
	defer local.Close()
	info, e := os.Stat(filepath.Join(dir, "agent-endpoint.json"))
	if e != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatal("discovery permissions", e)
	}
	for _, origin := range []string{"", "https://evil.example"} {
		req, _ := http.NewRequest("POST", local.endpoint.URL, strings.NewReader(`{}`))
		if origin != "" {
			req.Header.Set("Authorization", "Bearer "+local.endpoint.Token)
			req.Header.Set("Origin", origin)
		}
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatal(res.Status)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server := NewMCP(func(ctx context.Context, n string, a Args) (any, error) { return RemoteCall(ctx, dir, n, a) })
	st, ct := mcp.NewInMemoryTransports()
	ss, e := server.Connect(ctx, st, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, e := client.Connect(ctx, ct, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer cs.Close()
	list, e := cs.ListTools(ctx, nil)
	if e != nil || len(list.Tools) != 9 {
		t.Fatal(list, e)
	}
	result, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: "capture_status", Arguments: map[string]any{}})
	if e != nil || result.IsError {
		t.Fatal(result, e)
	}
	result, e = cs.CallTool(ctx, &mcp.CallToolParams{Name: "read_body", Arguments: map[string]any{}})
	if e == nil && !result.IsError {
		t.Fatal("missing required fields accepted")
	}
	id := addFlow(t, s, "api.test", `{"ok":true}`)
	result, e = cs.CallTool(ctx, &mcp.CallToolParams{Name: "read_body", Arguments: map[string]any{"flow_id": id, "direction": "response"}})
	if e != nil || result.IsError {
		t.Fatal(result, e)
	}
	local.Close()
	if _, e = os.Stat(local.path); !os.IsNotExist(e) {
		t.Fatal("endpoint left behind")
	}
}
