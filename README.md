# mitmAny

**Inspect desktop and Android HTTP / HTTPS traffic, then use real requests as evidence for AI-assisted analysis.**

An open-source desktop traffic inspector built with Go, Wails and React for macOS and Windows. Capture through the system proxy or an Android ADB USB tunnel. Inspect locally, or connect a compatible AI client through the read-only MCP bridge.

[简体中文](README.zh-CN.md) · [Downloads](https://github.com/wonderfulsuccess/mitmAny/releases) · [Installation](docs/installation.md) · [Contributing](CONTRIBUTING.md)

![Request list and JSON inspector with synthetic example traffic](docs/assets/workspace.png)

*This screenshot uses synthetic data. See the [demo walkthrough](docs/demo.md).*

## Downloads

**v1.2.0** · [Release notes](https://github.com/wonderfulsuccess/mitmAny/releases/tag/v1.2.0)

| Platform | Direct download |
| --- | --- |
| macOS — Apple Silicon and Intel | [Download universal ZIP](https://github.com/wonderfulsuccess/mitmAny/releases/download/v1.2.0/mitmAny-1.2.0-macos-universal.zip) |
| Windows — x64 | [Download Windows ZIP](https://github.com/wonderfulsuccess/mitmAny/releases/download/v1.2.0/mitmAny-1.2.0-windows-amd64.zip) |

[SHA-256 checksums](https://github.com/wonderfulsuccess/mitmAny/releases/download/v1.2.0/SHA256SUMS.txt). Extract before launching; move the macOS app to Applications. Windows requires WebView2. These packages are unsigned; macOS is not notarized. See [installation and verification](docs/installation.md).

## Language

English is the default language. Switch to **简体中文** in **Settings → Language**; your choice is saved for future launches. The [Chinese README](README.zh-CN.md) is maintained separately.

## Get started

1. Download a package from [Releases](https://github.com/wonderfulsuccess/mitmAny/releases). Move the macOS app to Applications; Windows requires WebView2.
2. Open **Settings → CA Certificate**, then install and trust your local CA to inspect HTTPS. Without trust, HTTPS is tunneled without decryption.
3. Start desktop capture, or connect an Android device with USB debugging enabled and turn on its proxy in the Android panel. Use your browser or app, then select a request.

Stop capture when finished. Restore the Android proxy before unplugging USB. Use the toolbar recovery button if desktop networking needs restoration. See [recovery and removal](docs/recovery.md) if the app cannot start.

## Features

- HTTP / HTTPS, HTTP/2, streaming responses and SSE prefixes.
- Domain grouping, filters, search, favorites, virtual lists, JSON folding and body search.
- Android USB capture without shared Wi-Fi.
- HAR import/export, copy as cURL, captured-body export.
- Local CA management, proxy backup/recovery, light and dark themes.
- Read-only MCP tools and a bundled protocol-analysis skill; connect through Settings → Codex.

## Limits and privacy

No TUN, UDP, QUIC or HTTP3 interception, certificate-pinning bypass, request replay or WebSocket message inspector. Clients must honor the proxy and trust the CA; Android apps may need explicit debug trust configuration.

Request and ordinary response capture: 1 MiB each. HTML response capture: 8 MiB. Decompression has the same limits. Retention: 3,000 requests and 256 MiB of body files. Busy queues may skip or truncate captures while forwarding continues. Observations do not pin evidence.

Traffic and CA keys stay local to mitmAny. **Connected AI clients may send tool results to their model provider.** Redaction covers common sensitive fields, not all possible secrets. HAR, cURL and body exports may contain credentials. Inspect only traffic you are authorized to access.

[Privacy](docs/privacy.md) · [Security reporting](SECURITY.md) · [Detailed Chinese manual](docs/manual.zh-CN.md)

## Development

Go 1.26+, Node.js from `.nvmrc`, Wails CLI 2.15.0 and native platform build tools are required.

```sh
nvm use
npm --prefix frontend ci
npm --prefix frontend test
npm --prefix frontend run build
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
wails dev
```

[Development](docs/development.md) · [Architecture](docs/architecture.md) · [Roadmap](docs/roadmap.md) · [Changelog](CHANGELOG.md) · [Releases](docs/releasing.md)

## Support the project

If mitmAny is useful to you, you can leave an optional, one-time tip to support ongoing maintenance:

[US $1](https://buy.stripe.com/aFa7sMaQP5TIgZr7Xt97G02) · [US $10](https://buy.stripe.com/28E28s3oneqe6kNcdJ97G03) · [US $100](https://buy.stripe.com/fZu00k3on0zo8sV2D997G04)

These are one-time payments, not subscriptions. Support does not unlock paid features or priority support. The Stripe checkout merchant is **SimiSF**.

Prices are set in USD and include applicable taxes. Stripe may offer a local-currency amount; review the currency and total before paying. Available payment methods depend on your region and checkout currency. Alipay and WeChat Pay were available in the CNY checkout when verified; WeChat Pay was not available in USD. The options shown at checkout take precedence.

Bug reports, documentation improvements and code contributions are also welcome.

## License

[MIT](LICENSE). Vendored dependencies retain their own licenses. See [third-party notices](THIRD_PARTY_NOTICES.md).
