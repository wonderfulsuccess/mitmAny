package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"mitmAny/internal/android"
)

func exitApp(t *testing.T) (*App, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake ADB uses a POSIX shell")
	}
	dir := t.TempDir()
	t.Setenv("MITMANY_EXIT_TEST_DIR", dir)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$MITMANY_EXIT_TEST_DIR/calls"
if [ "$MITMANY_EXIT_TEST_OFFLINE" = "1" ]; then echo 'device offline' >&2; exit 1; fi
case "$*" in
 *'settings delete global http_proxy') printf 'null\n' > "$MITMANY_EXIT_TEST_DIR/phone" ;;
 *'settings get global http_proxy') cat "$MITMANY_EXIT_TEST_DIR/phone" ;;
 *'reverse --list') printf 'UsbFfs tcp:9081 tcp:9081\n' ;;
 *'reverse --remove tcp:9081') printf 'removed\n' > "$MITMANY_EXIT_TEST_DIR/tunnel" ;;
esac
`
	for name, body := range map[string]string{
		"adb":                  script,
		"phone":                android.Proxy,
		"android-proxies.json": `{"abc":{"proxy":"null","reverse":true}}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	manager, err := android.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return &App{android: manager, running: true, mobileSerial: "abc"}, dir
}

func TestBeforeCloseRestoresPhoneAutomatically(t *testing.T) {
	app, dir := exitApp(t)
	if prevent := app.beforeClose(context.Background()); prevent {
		t.Fatal("successful cleanup blocked exit")
	}
	phone, _ := os.ReadFile(filepath.Join(dir, "phone"))
	tunnel, _ := os.ReadFile(filepath.Join(dir, "tunnel"))
	if strings.TrimSpace(string(phone)) != "null" || strings.TrimSpace(string(tunnel)) != "removed" {
		t.Fatal("phone routing not cleaned up")
	}
	if app.running || len(app.android.Pending()) != 0 || app.mobileSerial != "" {
		t.Fatal("application not cleaned up")
	}
	calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
	if strings.Index(string(calls), "settings delete") > strings.Index(string(calls), "reverse --remove") {
		t.Fatal("tunnel removed before proxy restoration")
	}
	// Wails invokes shutdown after beforeClose; repeated cleanup must be harmless.
	app.shutdown(context.Background())
}

func TestPrepareExitRetainsOfflineDeviceAndStillStopsDesktopCapture(t *testing.T) {
	app, _ := exitApp(t)
	t.Setenv("MITMANY_EXIT_TEST_OFFLINE", "1")
	if err := app.prepareExit(); err == nil {
		t.Fatal("offline phone was silently ignored")
	}
	if len(app.android.Pending()) != 1 {
		t.Fatal("offline phone backup lost")
	}
	if app.running {
		t.Fatal("Android failure skipped desktop cleanup")
	}
	t.Setenv("MITMANY_EXIT_TEST_OFFLINE", "0")
	if err := app.prepareExit(); err != nil {
		t.Fatal(err)
	}
}
