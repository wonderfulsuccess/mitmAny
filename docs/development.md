# Development

## Requirements

- Go 1.26+ (the exact language version is in `go.mod`).
- Node.js 22.20.0 (`.nvmrc`) and npm. With nvm: `nvm install && nvm use`.
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0`.
- macOS: Xcode Command Line Tools (`xcode-select --install`).
- Windows: native build prerequisites reported by `wails doctor`, plus WebView2 Runtime.
- Optional Android testing: SDK Platform-Tools / ADB and a dedicated test device.

```sh
npm --prefix frontend ci
npm --prefix frontend run build
wails doctor
wails dev
```

The frontend build must exist before Go builds/tests because `main.go` embeds `frontend/dist`. Generated Wails bindings are ignored. Do not commit runtime data or build outputs.

## Checks

```sh
node scripts/check-release.mjs
npm --prefix frontend test
npm --prefix frontend run build
go test -race ./...
go vet ./...
```

Build the frontend before Go checks; do not rebuild `frontend/dist` concurrently with Go compilation. `go test ./...` is the portable baseline; race tests run on macOS CI. The vendored module is a separate Go module: run its tests from `third_party/go-mitmproxy` when editing the engine. No test should depend on real credentials or public websites.

Optional Android device test (temporarily changes/restores the selected device's proxy):

```sh
MITMANY_TEST_ANDROID_SERIAL=<test-device-serial> go test -race -run TestAndroidUSBProxy .
```

For UI-only work: `npm --prefix frontend run dev -- --host 127.0.0.1`. The browser preview cannot capture traffic. `http://127.0.0.1:5173/tests/open-source-demo.html` uses synthetic data and mocks native operations; it never installs a CA or changes proxies.

## Builds

`wails build -clean` builds the host app. macOS: `wails build -clean -platform darwin/universal`. The local `scripts/build-release.sh` creates separate Intel, Apple Silicon and Windows ZIPs from a temporary copy. CI creates a universal macOS ZIP and a Windows x64 ZIP.

See [release procedure](releasing.md) and [manual platform acceptance](release-checklist.md). A cross-compiled Windows binary is not a substitute for Windows device testing.

First-capture UI fixtures: append `?empty` to the synthetic demo for an untrusted CA and empty request list. Add `&setup=install-error`, `start-error`, `check-error`, `invalid`, `missing`, or `ready` for setup edge cases. All operations remain mocked. The `mitmAny:capture-setup:v1` local-storage key records dismissal; certificate problems still prompt once on each app launch.
