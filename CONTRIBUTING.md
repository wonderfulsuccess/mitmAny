# Contributing

Thank you for helping improve mitmAny. Issues and pull requests in English or Chinese are welcome.

## Before making a change

- Read [development](docs/development.md), [architecture](docs/architecture.md) and the [roadmap](docs/roadmap.md).
- Discuss protocol interception, trust, storage and system proxy changes in an issue before a large implementation.
- Report vulnerabilities privately using [SECURITY.md](SECURITY.md).
- Use synthetic hosts such as `example.test` and generated fixtures. Never attach live HAR files, tokens, private keys or device identifiers.

## Local workflow

1. Fork and clone the repository; create a topic branch.
2. Install the documented toolchain and run the existing tests.
3. Make a focused change. Add regression coverage for behavior changes, especially forwarding and recovery failures.
4. Run `npm --prefix frontend test`, `npm --prefix frontend run build`, `go test -race ./...` and `go vet ./...` on macOS. CI also builds/tests Windows.
5. Update relevant documentation and `CHANGELOG.md` under Unreleased.
6. Open a pull request explaining the problem, resulting behavior and validation. Identify untested platform behavior.

Use `gofmt` for Go. Follow existing TypeScript/CSS conventions; avoid unrelated formatting changes. Keep third-party engine patches small and document them in `third_party/go-mitmproxy/PATCHES.md`.

Do not run device/system integration tests on another person's machine without their consent. The Android integration test changes the selected device's proxy and is opt-in.

Contributions are submitted under the project's MIT license. Keep original copyright and license notices when incorporating third-party work. Be respectful, provide reproducible evidence, and focus review on the code.
