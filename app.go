package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/lqqyt2423/go-mitmproxy/cert"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	log "github.com/sirupsen/logrus"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"mitmAny/internal/android"
	"mitmAny/internal/capture"
	"mitmAny/internal/certificates"
	"mitmAny/internal/platform"
)

type App struct {
	android        *android.Manager
	mobileEngine   *proxy.Proxy
	mobileListener net.Listener
	mobileSerial   string
	ctx            context.Context
	mu             sync.Mutex
	engine         *proxy.Proxy
	listener       net.Listener
	observer       *capture.Observer
	store          *capture.Store
	certificates   *certificates.Manager
	system         *platform.ProxyManager
	dir            string
	running        bool
	systemEnabled  bool
	recording      bool
	intercept      atomic.Bool
	upstream       atomic.Pointer[platform.UpstreamConfig]
	lastError      string
	initError      string
}
type AppState struct {
	Running         bool   `json:"running"`
	SystemProxy     bool   `json:"systemProxy"`
	Recording       bool   `json:"recording"`
	Address         string `json:"address"`
	Platform        string `json:"platform"`
	RecoveryPending bool   `json:"recoveryPending"`
	Error           string `json:"error"`
	StorageError    string `json:"storageError"`
	DataDirectory   string `json:"dataDirectory"`
}

func NewApp() *App { return &App{recording: true} }
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	log.SetLevel(log.ErrorLevel)
	dir, err := os.UserConfigDir()
	if err != nil {
		a.initError = err.Error()
		return
	}
	a.dir = filepath.Join(dir, "mitmAny")
	if err = os.MkdirAll(a.dir, 0700); err != nil {
		a.initError = err.Error()
		return
	}
	a.android, err = android.New(a.dir)
	if err != nil {
		a.initError = err.Error()
		return
	}
	a.system = platform.NewProxyManager(a.dir)
	// Recover the exact proxy configuration saved by an interrupted previous run.
	if a.system.Pending() {
		if err = a.system.Restore(); err != nil {
			a.lastError = "上次的代理配置恢复失败: " + err.Error()
		}
	}
	a.certificates, err = certificates.New(filepath.Join(a.dir, "ca"))
	if err != nil {
		a.initError = err.Error()
		return
	}
	a.store, err = capture.NewStore(filepath.Join(a.dir, "session"))
	if err != nil {
		a.initError = err.Error()
		return
	}
	a.observer = capture.NewObserver(a.store)
}
func (a *App) GetState() AppState { a.mu.Lock(); defer a.mu.Unlock(); return a.stateLocked() }
func (a *App) stateLocked() AppState {
	s := AppState{Running: a.running, SystemProxy: a.systemEnabled, Recording: a.recording, Address: "127.0.0.1:9080", Platform: runtime.GOOS, Error: a.lastError, DataDirectory: a.dir}
	if a.initError != "" {
		s.Error = a.initError
	}
	if a.system != nil {
		s.RecoveryPending = a.system.Pending() && !a.systemEnabled
	}
	if a.store != nil {
		s.StorageError = a.store.Error()
	}
	return s
}
func (a *App) StartCapture(systemProxy bool) (AppState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.initError != "" {
		return a.stateLocked(), errors.New(a.initError)
	}
	if a.running {
		return a.stateLocked(), nil
	}
	if systemProxy && !a.certificates.Status().Trusted {
		return a.stateLocked(), errors.New("请先在证书管理中安装并信任 CA，再接管系统 HTTPS 流量。也可使用手动代理模式。")
	}
	upstream := platform.UpstreamConfig{}
	if systemProxy {
		var err error
		upstream, err = a.system.Upstream()
		if err != nil {
			return a.stateLocked(), err
		}
	}
	for _, scheme := range []string{"http", "https"} {
		u, _ := url.Parse(scheme + "://mitmany.invalid")
		if _, err := upstream.Resolve(&http.Request{URL: u}); err != nil {
			return a.stateLocked(), err
		}
	}
	a.upstream.Store(&upstream)
	a.intercept.Store(a.certificates.Status().Trusted)
	if a.engine != nil {
		if systemProxy {
			if err := a.system.Enable("127.0.0.1", 9080); err != nil {
				return a.stateLocked(), err
			}
			a.systemEnabled = true
		}
		a.running = true
		a.observer.SetRecording(a.recording)
		a.lastError = ""
		return a.stateLocked(), nil
	}
	listener, err := net.Listen("tcp", "127.0.0.1:9080")
	if err != nil {
		return a.stateLocked(), fmt.Errorf("代理端口 9080 不可用: %w", err)
	}
	engine, err := proxy.NewProxy(&proxy.Options{Addr: listener.Addr().String(), ReadOnlyCapture: true, NewCaFunc: func() (cert.CA, error) { return a.certificates.CA, nil }})
	if err != nil {
		listener.Close()
		return a.stateLocked(), err
	}
	// Preserve the previous system proxy while rejecting recursive environment proxies.
	engine.SetUpstreamProxy(func(req *http.Request) (*url.URL, error) { return a.upstream.Load().Resolve(req) })
	engine.SetShouldInterceptRule(func(*http.Request) bool { return a.intercept.Load() })
	engine.AddAddon(proxy.NewUpstreamCertAddon(true))
	engine.AddAddon(a.observer)
	a.engine = engine
	a.listener = listener
	a.running = true
	a.observer.SetRecording(a.recording)
	a.lastError = ""
	go func() {
		err := engine.Serve(listener)
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.engine != engine {
			return
		}
		if a.systemEnabled {
			if restoreErr := a.system.Restore(); restoreErr != nil {
				a.lastError = restoreErr.Error()
			} else {
				a.systemEnabled = false
			}
		}
		a.running = false
		a.engine = nil
		a.observer.SetRecording(a.recording && a.mobileEngine != nil)
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
			a.lastError = err.Error()
		}
	}()
	if systemProxy {
		if err = a.system.Enable("127.0.0.1", 9080); err != nil {
			a.running = false
			a.engine = nil
			listener.Close()
			engine.Close()
			return a.stateLocked(), err
		}
		a.systemEnabled = true
	}
	return a.stateLocked(), nil
}
func (a *App) StopCapture() (AppState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	// Restore routing before closing the listener. On failure leave proxy serving.
	if a.systemEnabled || a.system != nil && a.system.Pending() {
		if err := a.system.Restore(); err != nil {
			return a.stateLocked(), err
		}
		a.systemEnabled = false
	}
	// Keep forwarding existing connections, including downloads and SSE. The listener
	// stays available for clients caching system proxy settings until the app exits.
	if a.observer != nil {
		a.observer.SetRecording(a.recording && a.mobileEngine != nil)
	}

	a.running = false
	return a.stateLocked(), nil
}
func (a *App) RecoverNetwork() (AppState, error) { return a.StopCapture() }
func (a *App) SetRecording(recording bool) AppState {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.recording = recording
	if a.observer != nil {
		a.observer.SetRecording(recording && (a.running || a.mobileEngine != nil))
	}
	return a.stateLocked()
}
func (a *App) GetFlows(cursor uint64) capture.Batch {
	if a.store == nil {
		return capture.Batch{Flows: []capture.Summary{}}
	}
	return a.store.Batch(cursor, a.observer.Dropped())
}
func (a *App) GetFlow(id string) (capture.Detail, error) {
	if a.store == nil {
		return capture.Detail{}, errors.New(a.initError)
	}
	return a.store.Detail(id)
}
func (a *App) ClearFlows() {
	if a.store != nil {
		a.observer.Clear()
	}
}
func (a *App) GetCertificateStatus() certificates.Status {
	if a.certificates == nil {
		return certificates.Status{Error: a.initError}
	}
	return a.certificates.Status()
}
func (a *App) InstallCertificate() (certificates.Status, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.certificates == nil {
		return certificates.Status{}, errors.New(a.initError)
	}
	if a.running || a.mobileEngine != nil {
		return a.certificates.Status(), errors.New("请先停止抓包，再安装或重新安装证书")
	}
	err := a.certificates.Install()
	status := a.certificates.Status()
	a.intercept.Store(status.Trusted)
	return status, err
}
func (a *App) UninstallCertificate() (certificates.Status, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.certificates == nil {
		return certificates.Status{}, errors.New(a.initError)
	}
	if a.running || a.mobileEngine != nil {
		return a.certificates.Status(), errors.New("请先停止抓包再卸载 CA")
	}
	err := a.certificates.Uninstall()
	status := a.certificates.Status()
	a.intercept.Store(status.Trusted)
	return status, err
}
func (a *App) ExportCertificate() error {
	if a.certificates == nil {
		return errors.New(a.initError)
	}
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{DefaultFilename: "mitmAny-ca.pem", Title: "导出 CA 公钥证书"})
	if err != nil || path == "" {
		return err
	}
	return a.certificates.Export(path)
}
func (a *App) SaveBody(id string, request bool) error {
	d, err := a.GetFlow(id)
	if err != nil {
		return err
	}
	body := d.ResponseBody
	headers, encoding := d.ResponseHeaders, d.ResponseEncoding
	if request {
		body = d.RequestBody
		headers, encoding = d.RequestHeaders, d.RequestEncoding
	}
	data, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return err
	}
	filename := bodyFilename(d.URL, headers, encoding, data, request)
	ext := filepath.Ext(filename)
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{DefaultFilename: filename, Title: "保存已捕获的正文", Filters: []wruntime.FileFilter{{DisplayName: "正文文件 (" + ext + ")", Pattern: "*" + ext}, {DisplayName: "所有文件", Pattern: "*"}}})
	if err != nil || path == "" {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
func (a *App) ExportHAR() error {
	if a.store == nil {
		return errors.New(a.initError)
	}
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{DefaultFilename: "mitmAny-" + time.Now().Format("20060102-150405") + ".har", Title: "导出 HAR", Filters: []wruntime.FileFilter{{DisplayName: "HTTP Archive", Pattern: "*.har"}}})
	if err != nil || path == "" {
		return err
	}
	data, err := a.harData()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (a *App) harData() ([]byte, error) {
	batch := a.store.Batch(0, 0)
	entries := make([]any, 0, len(batch.Flows))
	headers := func(h http.Header) []any {
		out := []any{}
		for k, values := range h {
			for _, v := range values {
				out = append(out, map[string]string{"name": k, "value": v})
			}
		}
		return out
	}
	for _, summary := range batch.Flows {
		d, err := a.store.Detail(summary.ID)
		if err != nil {
			continue
		}
		u, _ := url.Parse(d.URL)
		query := []any{}
		if u != nil {
			for k, values := range u.Query() {
				for _, v := range values {
					query = append(query, map[string]string{"name": k, "value": v})
				}
			}
		}
		meta := map[string]any{"truncated": d.Truncated, "error": d.Error, "requestEncoding": d.RequestEncoding, "responseEncoding": d.ResponseEncoding, "requestTrailers": d.RequestTrailers, "responseTrailers": d.ResponseTrailers}
		request := map[string]any{"method": d.Method, "url": d.URL, "httpVersion": d.Protocol, "headers": headers(d.RequestHeaders), "queryString": query, "cookies": []any{}, "headersSize": -1, "bodySize": d.RequestSize}
		if d.RequestBody != "" {
			body, _ := base64.StdEncoding.DecodeString(d.RequestBody)
			text := string(body)
			if !utf8.Valid(body) {
				meta["requestBodyBase64"] = d.RequestBody
				text = ""
			}
			request["postData"] = map[string]any{"mimeType": d.RequestHeaders.Get("Content-Type"), "text": text}
		}
		entry := map[string]any{"startedDateTime": time.UnixMilli(d.Started).UTC().Format(time.RFC3339Nano), "time": d.Duration, "request": request, "response": map[string]any{"status": d.Status, "statusText": http.StatusText(d.Status), "httpVersion": d.Protocol, "headers": headers(d.ResponseHeaders), "cookies": []any{}, "content": map[string]any{"size": d.ResponseSize, "mimeType": d.ContentType, "text": d.ResponseBody, "encoding": "base64"}, "redirectURL": d.ResponseHeaders.Get("Location"), "headersSize": -1, "bodySize": d.ResponseSize}, "cache": map[string]any{}, "timings": map[string]any{"send": 0, "wait": d.TTFB, "receive": max(0, d.Duration-d.TTFB)}, "_mitmAny": meta}
		entries = append(entries, entry)
	}
	return json.MarshalIndent(map[string]any{"log": map[string]any{"version": "1.2", "creator": map[string]string{"name": "mitmAny", "version": "1.1.0"}, "entries": entries}}, "", "  ")
}
func (a *App) WindowAction(action string) {
	switch action {
	case "minimise":
		wruntime.WindowMinimise(a.ctx)
	case "maximise":
		wruntime.WindowToggleMaximise(a.ctx)
	case "close":
		wruntime.Quit(a.ctx)
	}
}

// Restore phone routing before stopping its listener, and always attempt desktop
// restoration independently so one failure cannot skip the other cleanup.
func (a *App) prepareExit() error {
	a.mu.Lock()
	androidErr := a.stopAndroidLocked()
	a.mu.Unlock()
	_, desktopErr := a.StopCapture()
	return errors.Join(androidErr, desktopErr)
}
func (a *App) beforeClose(ctx context.Context) bool {
	err := a.prepareExit()
	if err == nil {
		return false
	}
	_, _ = wruntime.MessageDialog(ctx, wruntime.MessageDialogOptions{Type: wruntime.ErrorDialog, Title: "网络配置尚未恢复", Message: "代理恢复失败，请连接手机并恢复手机代理，或点击恢复网络后重试。\n" + err.Error()})
	return true
}
func (a *App) shutdown(context.Context) {
	_ = a.prepareExit()
	a.mu.Lock()
	engine := a.engine
	a.engine = nil
	listener := a.listener
	a.mu.Unlock()
	if listener != nil {
		listener.Close()
	}
	if engine != nil {
		engine.Close()
	}
	if a.observer != nil {
		a.observer.Close()
	}
	if a.store != nil {
		a.store.Flush()
	}
}

func (a *App) ImportHAR() (int, error) {
	if a.store == nil {
		return 0, errors.New(a.initError)
	}
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "导入 HTTP Archive", Filters: []wruntime.FileFilter{{DisplayName: "HTTP Archive", Pattern: "*.har;*.json"}}})
	if err != nil || path == "" {
		return 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if info.Size() > 64*1024*1024 {
		return 0, errors.New("单次导入的 HAR 文件不能超过 64 MiB")
	}
	return a.observer.ImportHAR(io.LimitReader(f, 64*1024*1024))
}

// OpenCertificateInKeychain exports only the public CA, never the private key.
func (a *App) OpenCertificateInKeychain() error {
	if a.certificates == nil {
		return errors.New(a.initError)
	}
	if runtime.GOOS != "darwin" {
		return errors.New("此入口仅用于 macOS 钥匙串")
	}
	if err := a.certificates.Export(a.certificates.Path()); err != nil {
		return err
	}
	return exec.Command("/usr/bin/open", "-b", "com.apple.keychainaccess", a.certificates.Path()).Run()
}
