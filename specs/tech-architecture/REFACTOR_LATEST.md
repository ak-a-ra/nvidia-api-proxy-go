# Refactor and Hardening Plan

## Reason for Existence

This plan converts the completed parity port into a safer, easier-to-test proxy without adding dependencies or changing the documented HTTP contract.

## Current Behavior

Startup loads environment configuration, binds one HTTP server, and registers one SIGTERM shutdown path. `NewHandler` owns a shared HTTP client. Its closure performs routing and authentication, constructs upstream requests, rewrites headers and credentials, forwards response headers and status, then streams response bodies with a resettable idle timer.

## Problems

- Routing and forwarding use different path representations, allowing dot-segment ambiguity.
- The connect timeout starts too late to cover dialing and request-body upload.
- The gzip idle timer closes only the decoder, not the blocked transport body.
- Unsupported response encodings lose their header while their compressed bytes remain unchanged.
- Connection-nominated hop-by-hop headers survive.
- Token digests are recomputed for every authenticated request.
- Timeout literals, response framing, and three identical 502 branches obscure policy.
- Tests share captured variables across handler goroutines and miss key timeout and compression edges.

## Improved Architecture

Use one internal `proxyHandler` module that owns the immutable configuration snapshot, shared HTTP client, and precomputed token digest. Keep `NewHandler` as the compatibility interface. Route with one normalized target derived from the request. Separate request construction, response preparation, header transfer, and streaming internally. Keep all public behavior observable through `http.Handler`.

## Invariants

- No third-party dependency, retry, rate limit, or response buffering is introduced.
- Secrets and upstream error details never enter responses or logs.
- Request context cancellation always reaches upstream work.
- Streaming begins before the upstream body completes.
- Multi-value `Set-Cookie` values remain distinct.

## Implementation Steps

1. Add routing regressions for encoded and literal dot segments → verify: `go test -run 'TestProxyRoutingNormalizes' ./...`
2. Normalize one escaped path for routing and forwarding → verify: `go test -run 'TestProxyRouting' ./...`
3. Add connection-nominated header regressions → verify: `go test -run 'TestProxyHeaders' ./...`
4. Strip all per-hop connection fields in both directions → verify: `go test ./...`
5. Add stalled-upload and gzip-stall timeout regressions → verify: `go test -run 'TestConnectTimeout|TestIdleTimeout' ./...`
6. Cover the complete pre-response operation and close the underlying gzip body on idle → verify: `go test -run 'TestConnectTimeout|TestIdleTimeout' ./...`
7. Add unsupported-encoding and bodyless-framing regressions → verify: `go test -run 'TestUpstream|TestNoContent|TestHead' ./...`
8. Preserve unsupported encodings and restore bodyless content lengths only when valid → verify: `go test ./...`
9. Introduce the internal handler state, precomputed digest, named constants, and one 502 writer → verify: `go test ./...`
10. Harden timeout range parsing and remove dead configuration state → verify: `go test ./...`
11. Remove test data races by publishing upstream observations through channels → verify: `go test -race ./...`
12. Refresh architecture and verification evidence → verify: `go test ./... && go vet ./... && go build ./...`

## Test Decisions

- Assert observable HTTP behavior through `NewHandler` and real local servers.
- Add deterministic tests for dot segments, header nomination, upload timeout, gzip stall, unsupported encodings, and bodyless framing.
- Replace cross-goroutine shared captures with channels before enabling the race detector.
- Keep the existing test suite as the parity safety net for routing order, auth, cookies, streaming, cancellation, and shutdown.

## Out of Scope

- Adding retries, rate limiting, or new endpoints.
- Adding third-party packages.
- Buffering SSE or other response bodies.
- Splitting the small `main` package into new public packages.
- Rewriting process-level startup and shutdown architecture before another measured bottleneck exists.

## Verification

Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and `go build ./...`.

## Senior Review Pass — 2026-09-25

### Architecture summary

- `config.go` parses immutable process configuration and timeout policy.
- `main.go` owns the listener, SIGTERM handling, and bounded shutdown.
- `proxy.go` owns routing, authentication, request construction, and response orchestration.
- `proxy_headers.go` owns hop-by-hop filtering and response framing.
- `proxy_stream.go` owns body preparation, gzip ownership, idle timers, and streaming.

Request flow: normalized escaped path and raw query become one target; health, route, and authentication checks run in order; the request context reaches the shared HTTP client; upstream authorization and identity encoding replace caller headers. Response flow: body preparation decides pass-through, gzip decode, or bodyless handling; framing is copied before status; the body streams through a pooled buffer with a resettable idle timer.

### Findings

- **Structural:** routing, transport, and response orchestration remain coupled in `proxy.go`; the module is still small enough that package decomposition would add navigation cost.
- **Duplication:** fixed JSON responses were repeated map literals; response-body flags were positional booleans; fixed defaults and shutdown policy used magic values.
- **Performance:** token hashing and buffer allocation were already optimized. Header nomination allocated a map for every message, and JSON responses marshaled a map per local reply.
- **Maintainability/security:** custom dot-segment normalization needs edge coverage; response `Content-Length` needed validation; short downstream writes needed explicit failure handling; server slow-header policy and active-shutdown completion remain deployment/test risks.

### Implemented refactor

- Replaced local JSON map marshaling with exact payload constants and one writer.
- Replaced positional response-body returns with `preparedResponseBody`.
- Dropped malformed or ambiguous upstream `Content-Length` values before forwarding.
- Made short downstream writes return `io.ErrShortWrite`, preventing silent truncation.
- Avoided connection-header map allocation when no `Connection` header exists.
- Named default port, timeout, and shutdown values.
- Added regression tests for invalid framing and short writes.

### Deferred strategy

Keep one `main` package, no response buffering, no new dependencies. Add server read-header deadlines, active-shutdown completion tests, and a measured transport-defaults change only when deployment policy or profiling requires them.
