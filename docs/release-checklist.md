# Release acceptance checklist

Copy this checklist into each release review. Unchecked means not yet verified, not a known failure. Record OS version, hardware, app version, archive checksum and tester/date for each platform.

## Repository and distribution

- [x] Existing application-icon public redistribution permission confirmed by the maintainer on 2026-11-18.
- [ ] Confirm licensing authority for any newly added third-party assets in this release.
- [ ] Review complete Git history and current source with secret scanning; resolve real findings.
- [ ] Run CI on GitHub on the exact release commit (macOS + Windows).
- [ ] Confirm versions and new tag, changelog, third-party license bundle and hashes.
- [ ] Confirm public security contact and enable private vulnerability reporting.
- [ ] Enable branch protection / required checks; review Actions permissions and signing secret access.
- [ ] Review signed/notarized/unsigned status; install the exact ZIPs on clean machines.
- [ ] Verify README links/screenshots and replace draft release notes before publishing.

## Desktop acceptance — repeat on macOS Apple Silicon, Intel, Windows x64

- [ ] Fresh installation, first launch, expected OS prompts and WebView2 dependency.
- [ ] Install CA, inspect matching fingerprint, capture controlled HTTPS, uninstall trust.
- [ ] HTTP / HTTP2 / redirects / gzip / SSE preserve traffic bytes and behavior.
- [ ] HTML above 8 MiB and compressed expansion above 8 MiB remain bounded; show capture limitation.
- [ ] Stop/normal quit restores original direct-connect and explicit upstream-proxy settings.
- [ ] Force terminate while capturing, relaunch, recover; test documented manual recovery.
- [ ] Sleep/wake, switch Wi-Fi/network services, existing PAC and VPN coexistence; record unsupported combinations.
- [ ] Disk full/write failure, queue pressure and retention eviction surface correctly without corrupting evidence.
- [ ] HAR import/export, body save, cURL, keyboard controls, narrow window and both themes.

## Android — dedicated test devices

- [ ] ADB unauthorized/device/offline states, USB reverse tunnel, controlled debug app HTTPS.
- [ ] Trust installation only copies the public CA; pinned/non-trusting apps show expected limitations.
- [ ] Disable proxy before unplugging; unexpected disconnect/reconnect restores original proxy.
- [ ] Multiple pending devices, existing tunnel conflict, exit with offline device, retry restoration.

## MCP and removal

- [ ] Install/connect, preserve unrelated config, reopen client, query synthetic traffic.
- [ ] Reject invalid token/Host/Origin; verify common sensitive-field masking and invalid JSON handling.
- [ ] Display scope, truncation, eviction/reset and AI data-sharing limitations accurately.
- [ ] Remove only this integration, desktop/Android CA trust, app and local data; preserve unrelated settings.
