# Privacy and data lifecycle

mitmAny's capture functionality stores data locally and has no built-in traffic-upload or analytics service. This does not restrict what connected AI clients do with MCP results.

| Data | Location / lifecycle |
| --- | --- |
| App data (macOS) | `~/Library/Application Support/mitmAny` |
| App data (Windows) | `%AppData%\mitmAny` |
| Session metadata and bodies | `session/`; saved periodically and restored on startup; Clear removes the local session |
| CA certificate and private key | `ca/`; retained across sessions; use certificate management to remove trust before deleting files |
| Desktop and Android proxy backups | `proxy-backup.json`, `android-proxies.json`; keep until restoration succeeds |
| MCP endpoint token | `agent-endpoint.json`; loopback only, rotated on restart and cleaned up on normal exit |
| Codex integration | Config under `CODEX_HOME` or `~/.codex`; skill under `~/.agents/skills/mitmany-network-analysis` |
| Exports and integration backups | Remain at their destination until manually deleted; Clear does not delete them |

Check the app's Settings for the actual data directory. POSIX permission modes are used for sensitive files; Windows access control follows the user's filesystem ACLs. Local storage is not encrypted by the app. Use OS account separation and disk encryption when needed.

## Retention and data quality

The rolling store retains at most 3,000 requests and 256 MiB of captured body bytes, including HTML. Metadata and filesystem overhead are additional. Request/ordinary response prefixes are capped at 1 MiB; HTML response prefixes at 8 MiB. Decompression has bounded output. Queue pressure can stop capture early without stopping forwarding. Session checkpoints may lag a crash. Clear and eviction invalidate evidence; MCP bookmarks do not preserve it.

## Sharing

HAR exports, copied cURL, URLs, screenshots, headers and saved bodies may include cookies, tokens, personal data and proprietary content. They are not automatically safe to publish. Use synthetic fixtures in issues.

MCP masks common sensitive header/query/JSON/form keys, but unknown fields and arbitrary text can still expose secrets. Tools return data to the connected client, which may send it to a remote model provider according to its own settings. Scope analysis to authorized traffic, clear unrelated sessions before capture, and review your client's data policy.

See [removal instructions](recovery.md) for CA trust, proxy restoration, integration removal and data cleanup.
