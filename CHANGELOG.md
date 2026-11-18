# Changelog

## 1.2.1 — 2026-11-18

- Add a visible toolbar support entry with optional $1/$10/$100 one-time Stripe links, in English and Simplified Chinese. All application features remain free.
- Rework README around AI-assisted API reverse engineering with MCP, original conceptual artwork and a workflow diagram.
- Add Traditional Chinese, Japanese and Korean README translations; bundle all five documentation languages. The application UI remains English/Simplified Chinese.


## 1.2.0 — 2026-11-18

- Default to English with persistent Simplified Chinese switching across the interface and native dialogs.
- Add optional one-time Stripe support links and direct Windows/macOS release downloads.

- Bound HTML capture and decompression to 8 MiB and include HTML in the 256 MiB body budget; keep 1 MiB request/ordinary-response limits.
- Stop waiting for HTML capture queues when forwarding; expose truncation reasons and bound reads of legacy body files.
- Apply body limits to HAR imports and align MCP/HAR/UI version reporting.
- Add first-capture guidance, a toolbar recovery action and AI data-sharing notices.
- Add MIT licensing, bilingual landing pages, synthetic demo, contribution/security/privacy/recovery documentation.
- Add frontend tests, release metadata checks, secret scanning and draft release automation with optional signing.

## 1.1.0

Existing pre-open-source baseline. Desktop/Android capture, local session inspection, CA management, HAR/cURL export and Codex/MCP integration. See Git history for implementation details; historical commit messages are preserved.
