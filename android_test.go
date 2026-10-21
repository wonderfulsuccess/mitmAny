package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mitmAny/internal/android"
	"mitmAny/internal/capture"
	"mitmAny/internal/certificates"
	"mitmAny/internal/platform"
)

func mobileApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	ca, err := certificates.New(filepath.Join(dir, "ca"))
	if err != nil {
		t.Fatal(err)
	}
	store, err := capture.NewStore(filepath.Join(dir, "session"))
	if err != nil {
		t.Fatal(err)
	}
	manager, err := android.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{android: manager, certificates: ca, store: store, observer: capture.NewObserver(store), system: platform.NewProxyManager(dir), recording: true}
	t.Cleanup(func() {
		a.mu.Lock()
		err := a.stopAndroidLocked()
		a.mu.Unlock()
		if err != nil {
			t.Error(err)
		}
		a.observer.Close()
	})
	return a
}
func TestAndroidHTTPSWithoutDesktopTrust(t *testing.T) {
	a := mobileApp(t)
	state, err := a.SetAndroidListening(true)
	if err != nil || !state.Listening {
		t.Fatal(state, err)
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "mobile-http") }))
	defer origin.Close()
	proxyURL, _ := url.Parse("http://" + android.Proxy)
	pool := x509.NewCertPool()
	pool.AddCert(a.certificates.Root())
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), TLSClientConfig: &tls.Config{RootCAs: pool}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	// A local HTTPS server's test root is added only to this engine's upstream pool.
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "mobile-https") }))
	defer secure.Close()
	upstreamPool := x509.NewCertPool()
	upstreamPool.AddCert(secure.Certificate())
	// The engine HTTP transport initializes its root pool from options when used.
	a.mobileEngine.Opts.UpstreamRootCAs = upstreamPool
	for _, u := range []string{origin.URL, secure.URL} {
		response, err := client.Get(u)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || !strings.HasPrefix(string(data), "mobile-") {
			t.Fatal(string(data), err)
		}
		if response.TLS != nil && response.TLS.PeerCertificates[0].Issuer.CommonName != a.certificates.Root().Subject.CommonName {
			t.Fatal("HTTPS was not intercepted")
		}
	}
	// Stopping desktop capture must keep mobile recording enabled.
	if _, err = a.StopCapture(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for a.GetFlows(0).Total < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := a.GetFlows(0); got.Total < 2 {
		t.Fatal("missing mobile flows", got.Total)
	}
	if a.GetState().Running {
		t.Fatal("mobile listener enabled desktop capture")
	}
	if state, err = a.SetAndroidListening(false); err != nil || state.Listening {
		t.Fatal(state, err)
	}
}

// Opt in only with an authorized USB device. Always restore its saved proxy.
func TestAndroidUSBProxy(t *testing.T) {
	serial := os.Getenv("MITMANY_TEST_ANDROID_SERIAL")
	if serial == "" {
		t.Skip("set MITMANY_TEST_ANDROID_SERIAL to exercise a real device")
	}
	a := mobileApp(t)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "mitmany-usb-ok") }))
	defer origin.Close()
	before, err := exec.Command("adb", "-s", serial, "shell", "settings", "get", "global", "http_proxy").Output()
	if err != nil {
		t.Fatal(err)
	}
	state, err := a.SetAndroidEnabled(serial, true)
	if err != nil || !state.Enabled || !state.Listening || state.Serial != serial {
		t.Fatal(state, err)
	}
	// Repeated enable must not overwrite the original proxy backup.
	if _, err = a.SetAndroidEnabled(serial, true); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("adb", "-s", serial, "shell", "curl", "--silent", "--show-error", "--max-time", "10", "--noproxy", "''", "--proxy", android.Proxy, origin.URL)
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "mitmany-usb-ok" {
		t.Fatalf("USB proxy: %s %v", out, err)
	}
	state, err = a.SetAndroidEnabled(serial, false)
	if err != nil || state.Enabled || state.Listening || len(state.Pending) != 0 {
		t.Fatal(state, err)
	}
	after, err := exec.Command("adb", "-s", serial, "shell", "settings", "get", "global", "http_proxy").Output()
	if err != nil || string(before) != string(after) {
		t.Fatalf("proxy not restored: before=%q after=%q err=%v", before, after, err)
	}
	// A conflicting tunnel must cause the whole enable operation to roll back.
	if out, err = exec.Command("adb", "-s", serial, "reverse", "--no-rebind", android.Endpoint, android.Endpoint).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v", out, err)
	}
	defer exec.Command("adb", "-s", serial, "reverse", "--remove", android.Endpoint).Run()
	state, err = a.SetAndroidEnabled(serial, true)
	if err == nil || state.Enabled || state.Listening || len(state.Pending) != 0 {
		t.Fatal("failed enable left proxy active", state, err)
	}
}

func TestAndroidEnableRequiresDevice(t *testing.T) {
	a := mobileApp(t)
	state, err := a.SetAndroidEnabled("", true)
	if err == nil || state.Enabled || state.Listening || len(state.Pending) != 0 {
		t.Fatal(state, err)
	}
}
