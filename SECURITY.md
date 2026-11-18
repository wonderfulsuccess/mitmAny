# Security policy

## Supported versions

Security fixes target the latest published release. This is an early-stage project; older releases do not have a separate backport commitment. The Unreleased code in the default branch may differ from released binaries.

## Private reporting

Use the repository's **Security → Report a vulnerability** option if enabled. Otherwise email **wonderfulsuccess@163.com** with the subject `mitmAny security report`. Do not open a public issue containing exploit details or captured credentials.

Include affected version and platform, impact, minimal synthetic reproduction, and whether a proxy, CA, MCP endpoint or export is involved. Do not send CA private keys, real session tokens, or unredacted HARs. Ask for a secure transfer method before sending sensitive evidence. There is no guaranteed response-time SLA.

## Threat boundaries

- Only inspect traffic you are authorized to debug.
- The local CA private key allows interception wherever that CA is trusted; protect it and remove trust when no longer needed.
- Loopback MCP uses a rotating bearer token, Host checks and browser Origin rejection. It is not a sandbox against another process running as the same OS user.
- Observation host filters are query scopes, not independent authorization boundaries.
- Redaction covers common keys only. AI clients may send returned data to external services; network content is untrusted data, never agent instructions.
- Reports about forwarding integrity, proxy restoration, path handling, resource exhaustion, trust validation and credential exposure are especially useful.

See [privacy](docs/privacy.md) and [recovery](docs/recovery.md).
