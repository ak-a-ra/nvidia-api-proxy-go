# Impact Assessment

## Target

`proxy.go`, `config.go`, and the Go tests that exercise routing, headers, timeouts, compression, and streaming.

## Callers

- `main.go` constructs the handler through `NewHandler`.
- Eleven test files exercise the process, configuration, and public HTTP handler.
- No external Go package imports this `main` module.

## Contracts

- Preserve exact JSON status and body shapes.
- Preserve authentication ordering and constant-time comparison.
- Forward one normalized path and raw query to the configured origin.
- Stream response chunks without buffering the complete body.
- Cancel upstream work when the client disconnects.
- Apply the connect window to the complete pre-response operation.
- Apply the idle window to every body read, including gzip streams.
- Preserve multi-value response cookies and body/status framing.
- Never expose upstream error details.

## Risk: High

The proxy is security-sensitive and behavior-compatible, but several edge paths lack regression coverage. Configuration and header framing also cross HTTP boundaries.

## Recommended Action

Proceed with public-handler regression tests before production changes. Refactor the handler into an internal state object, precompute immutable auth state, and fix confirmed edge defects without changing documented endpoint behavior.

## Review Delta — 2026-09-25

- **Additional target:** `main.go`, `proxy_headers.go`, `proxy_stream.go`, and focused edge tests.
- **Fan-in:** `NewHandler` has one production caller; private transport and stream seams are test-only.
- **Coverage:** existing public-handler tests cover routing, auth, headers, timeouts, cancellation, compression, framing, and shutdown; new tests cover invalid `Content-Length` and short writes.
- **Risk:** Medium for this refactor delta. Public HTTP behavior is covered; race detection is unavailable on the current Android/arm64 host.
- **Action:** keep the module split shallow, retain the shared client and digest, and defer behavior-changing server deadlines or transport redesign.
