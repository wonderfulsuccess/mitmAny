# Release procedure

## Prepare

1. Complete [release-checklist.md](release-checklist.md), including asset provenance and real-device acceptance. Do not mark untested items complete.
2. Set a new version in `wails.json`, `frontend/package.json`, the lockfile (including its root package) and `internal/version/version.go`. Run `node scripts/check-release.mjs`. UI, MCP and HAR metadata use these checked values. Existing 1.1.0 binaries must not be silently replaced with these Unreleased changes.
3. Move relevant Unreleased notes into a dated version entry in `CHANGELOG.md`.
4. Run frontend tests/build before Go tests, then `go test -race ./...`, `go vet ./...`, dependency/license collection and secret scans.
5. Commit the reviewed changes. Push a matching `vX.Y.Z` tag when ready to run release CI. Tag creation/push is a separate maintainer action.

## CI behavior

Every push/PR runs frontend tests/build, Go tests/vet, macOS race tests, dependency license collection, and desktop builds. A separate full-history Gitleaks job runs on all refs in the checkout. Tag/product version mismatches fail early.

A `v*` tag push creates **a draft GitHub Release only after both platforms and secret scanning pass**. Assets: universal macOS ZIP, Windows x64 ZIP, SHA256SUMS.txt and platform signing-status files. Download and smoke-test the exact attached archives, edit draft notes from CHANGELOG.md, then publish manually. A rerun will not overwrite an existing release; inspect/delete the incomplete draft intentionally before retrying if necessary.

No repository settings, secrets, tags or releases are created by local preparation scripts. Enable private vulnerability reporting and branch protection in GitHub before accepting external contributions.

## Optional signing secrets

Set these as repository/environment secrets, never files in Git. Signing steps only run on tag pushes, not pull requests. Use ephemeral hosted runners; do not expose signing secrets to untrusted branches or contributors.

macOS (all required together):

- `MACOS_CERTIFICATE_P12`: Base64 Developer ID Application certificate with private key.
- `MACOS_CERTIFICATE_PASSWORD`: P12 password.
- `MACOS_SIGNING_IDENTITY`: exact Developer ID Application identity.
- `APPLE_API_KEY_P8`: raw App Store Connect API private key for notarization.
- `APPLE_API_KEY_ID`, `APPLE_API_ISSUER_ID`: corresponding identifiers.

The script uses a temporary keychain, hardened runtime, timestamp, notarytool, stapling and signature assessment. Partial credentials fail the job; no credentials produce an explicitly unsigned package. Signing and notarization cannot be validated locally without your credentials.

Windows (both required together):

- `WINDOWS_CERTIFICATE_PFX`: Base64 exportable Authenticode certificate and private key.
- `WINDOWS_CERTIFICATE_PASSWORD`: PFX password.

The runner uses Windows SDK signtool and timestamping, then verifies the signature. Hardware-backed/cloud-signing certificates need a provider-specific integration; the PFX path does not support them. Missing credentials produce an explicitly unsigned package.

## Local packaging

On macOS, `./scripts/build-release.sh` builds three ZIPs in a temporary copy: macOS Intel, macOS Apple Silicon and Windows x64. It does not sign or notarize. Keep its separate-architecture naming distinct from CI's universal ZIP. Local cross-compilation does not replace native Windows CI/testing.

## Secret scanning

Install the pinned scanner used in CI:

```sh
go install github.com/zricethezav/gitleaks/v8@v8.24.3
gitleaks git --redact --log-opts="--all" .
```

Also scan a clean copy containing all tracked and intended-to-add files with `gitleaks dir --redact <copy>`. Do not indiscriminately scan local app data or print secret matches. Inspect findings privately; rotate any real exposed credential before deciding whether history needs rewriting. `.gitignore` does not remove existing history. Scanning is heuristic, not a guarantee.
