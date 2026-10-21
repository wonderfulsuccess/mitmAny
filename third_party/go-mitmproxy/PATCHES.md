Based on go-mitmproxy v1.9.4, MIT licensed (LICENSE retained).

- Serve(net.Listener) for atomic port binding and readiness.
- Close the internal TLS HTTP server, idle upstream connections, and internal listener on Close.
- Bound the shared upstream transport idle pool (512 total, 64 per host, 90 seconds).
- ReadOnlyCapture avoids SSE history/parsing and streams WebSocket messages with NextReader/NextWriter, avoiding ReadMessage's full-message allocation.
- Preserve upload ContentLength, Host, request trailers, and response trailers during streaming; guard the optional HTTP Flusher.
- Fall back to the CONNECT target for IP HTTPS requests with no SNI, both for downstream certificates and upstream hostname validation.
- Allow explicit upstream trust roots for isolated local HTTPS/HTTP2 tests; production leaves this nil to use system trust. Upstream verification remains enabled.
- Report TLS CONNECT failures to addon hooks.
- Name newly generated CAs mitmAny Local CA / mitmAny.

The rest of the engine remains upstream source. There is no automatic protocol bypass after a failed TLS handshake.

Network self-check fixes (2026-10-21):
- In read-only capture mode, relay normal WS/WSS and non-TLS CONNECT upgrades through the standard library's bidirectional ReverseProxy; preserve auth/cookies/subprotocols, record the 101 handshake, and retain buffered bytes. Message history remains disabled.
- Remove hop-by-hop request/response headers, preserving declared trailers and `TE: trailers`.
- Report downstream TLS certificate rejection and response-stream read/write failures to capture hooks; abort broken responses instead of terminating them as successful chunked bodies.
- Pool 32 KiB response forwarding buffers separately from the bounded capture block pool.
- Track accepted sockets and close hijacked CONNECT/WebSocket and standalone HTTP/2 sessions on Proxy.Close.
- Bound upstream connection establishment to 15 seconds, TLS handshakes to 10 seconds, and incoming HTTP headers to 15 seconds. Body transfer and SSE remain unbounded by an overall proxy timeout. Dial and TLS limits are configurable through Options.
