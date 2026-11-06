package agent

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

type Endpoint struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}
type LocalServer struct {
	lastActivity atomic.Int64
	server       *http.Server
	endpoint     Endpoint
	path         string
}
type call struct {
	Tool      string `json:"tool"`
	Arguments Args   `json:"arguments"`
}
type reply struct {
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func DefaultDir() (string, error) { d, e := os.UserConfigDir(); return filepath.Join(d, "mitmAny"), e }
func Listen(dir string, s *Service) (*LocalServer, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		listener.Close()
		return nil, err
	}
	ep := Endpoint{URL: "http://" + listener.Addr().String() + "/call", Token: hex.EncodeToString(key)}
	local := &LocalServer{endpoint: ep, path: filepath.Join(dir, "agent-endpoint.json")}
	host := listener.Addr().String()
	local.server = &http.Server{ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != host || r.Header.Get("Origin") != "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+ep.Token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != "POST" || r.URL.Path != "/call" {
			http.Error(w, "not found", 404)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		var c call
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&c); err != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		if err := dec.Decode(new(any)); err != io.EOF {
			http.Error(w, "invalid trailing input", 400)
			return
		}
		local.lastActivity.Store(time.Now().UnixMilli())
		result, err := s.Call(r.Context(), c.Tool, c.Arguments)
		out := reply{Result: result}
		if err != nil {
			out = reply{Error: err.Error()}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(out)
	})}
	data, _ := json.Marshal(ep)
	// Atomic discovery publication, with no token in stdout or command arguments.
	f, err := os.CreateTemp(dir, ".agent-endpoint-*")
	if err != nil {
		listener.Close()
		return nil, err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(data); err == nil {
		err = f.Close()
	} else {
		f.Close()
	}
	if err == nil {
		err = os.Rename(tmp, local.path)
	}
	if err != nil {
		listener.Close()
		return nil, err
	}
	go local.server.Serve(listener)
	return local, nil
}
func (s *LocalServer) Close() {
	// Do not remove discovery data belonging to a newer process.
	data, _ := os.ReadFile(s.path)
	var ep Endpoint
	if json.Unmarshal(data, &ep) == nil && ep.Token == s.endpoint.Token {
		os.Remove(s.path)
	}
	s.server.Close()
}
func RemoteCall(ctx context.Context, dir, tool string, a Args) (any, error) {
	data, err := os.ReadFile(filepath.Join(dir, "agent-endpoint.json"))
	if err != nil {
		return nil, errors.New("mitmAny is not available; open the updated desktop application first")
	}
	var ep Endpoint
	if err = json.Unmarshal(data, &ep); err != nil {
		return nil, err
	}
	u, err := url.Parse(ep.URL)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.Path != "/call" || u.RawQuery != "" || len(ep.Token) != 64 {
		return nil, errors.New("invalid local endpoint")
	}
	body, _ := json.Marshal(call{Tool: tool, Arguments: a})
	req, err := http.NewRequestWithContext(ctx, "POST", ep.URL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+ep.Token)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 35 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}
	defer client.CloseIdleConnections()
	res, err := client.Do(req)
	if err != nil {
		return nil, errors.New("cannot connect to mitmAny; reopen the desktop application")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("local service returned HTTP %d", res.StatusCode)
	}
	var out reply
	decoder := json.NewDecoder(io.LimitReader(res.Body, 4<<20))
	decoder.UseNumber()
	if err = decoder.Decode(&out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		return nil, errors.New(out.Error)
	}
	return out.Result, nil
}

// LastActivity is authenticated tool activity, not proof of a currently connected Codex session.
func (s *LocalServer) LastActivity() int64 { return s.lastActivity.Load() }
