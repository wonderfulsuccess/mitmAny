# Installation and verification

Download only from the project's [Releases](https://github.com/wonderfulsuccess/mitmAny/releases). Each release should state whether its packages are signed, notarized or unsigned; build success alone does not establish trust.

## macOS

Unzip the universal package, move `mitmAny.app` to `/Applications`, then launch it. Do not connect Codex from a temporary archive/DMG location. Signed releases should also be notarized and stapled. If macOS blocks an unsigned build, verify its origin/checksum first and follow the system's per-app approval flow only if you trust it. Do not disable Gatekeeper globally or run arbitrary quarantine-removal commands.

Optional verification of a signed download:

```sh
codesign --verify --deep --strict --verbose=2 /Applications/mitmAny.app
spctl --assess --type execute --verbose=2 /Applications/mitmAny.app
```

## Windows

Extract the ZIP before running `mitmAny.exe`. Install the [Microsoft Edge WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) if missing. SmartScreen reputation and Authenticode signing are separate; follow your organization's policy and verify the publisher for signed downloads.

```powershell
Get-AuthenticodeSignature .\mitmAny.exe | Format-List Status,SignerCertificate
```

## Checksums

Download `SHA256SUMS.txt` from the same release. On macOS, in the directory containing the release ZIPs:

```sh
shasum -a 256 -c SHA256SUMS.txt
```

If you downloaded only one ZIP, compute its hash with `shasum -a 256 <archive.zip>` and compare the matching line. On Windows:

```powershell
Get-FileHash .\mitmAny-<version>-windows-amd64.zip -Algorithm SHA256
```

Checksums detect corruption; an unsigned checksum file is not an independent publisher identity guarantee.

See [first capture](../README.md#get-started), [privacy](privacy.md) and [recovery](recovery.md).
