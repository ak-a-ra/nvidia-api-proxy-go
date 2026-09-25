# Tech Stack

## Runtime

- Go 1.24+
- Standard library only
- `net/http` server and client
- `render.yaml` deployment target

## Architecture

The executable uses one `main` package with five production modules:

- `config.go` owns environment parsing, validation, defaults, and process configuration.
- `main.go` owns listener startup, SIGTERM handling, and graceful server shutdown.
- `proxy.go` owns routing state, authentication state, upstream request lifecycle, and response forwarding.
- `proxy_headers.go` owns hop-by-hop filtering, connection-nominated header removal, cookies, encoding, and content-length framing.
- `proxy_stream.go` owns gzip preparation, pooled stream buffers, flushing, and resettable idle timeout behavior.

`NewHandler` remains the public construction seam. Internally, `proxyHandler` captures an immutable configuration snapshot, one shared HTTP client, and one precomputed token digest. The private `newHandler` seam accepts an HTTP client so transport-boundary tests can use deterministic adapters.

## Data Flow

1. `LoadConfig` validates `NVIDIA_BASE_URL`, derives the upstream origin, and reads credentials, port, and timeout policy.
2. `main` constructs the HTTP server and delegates request handling to `NewHandler`.
3. The handler derives one normalized escaped path and one raw-query target for both routing and forwarding.
4. `/health` returns local configuration health. Unknown paths return 404. `/v1/*` requires constant-time bearer authentication.
5. The upstream request receives the caller context, copied request headers, the NVIDIA API key, and `Accept-Encoding: identity`.
6. A stoppable pre-response timer bounds DNS, connection, upload, and response-header acquisition without canceling the response stream.
7. Confirmed gzip responses are decoded through a wrapper that closes the underlying transport body. Unsupported encodings pass through unchanged.
8. Response framing validates `Content-Length`, drops stale or ambiguous values, and copies status before the body loop streams chunks with flush and resettable idle timeout behavior.
9. Mid-stream source or destination failure aborts the downstream HTTP response instead of ending a truncated stream cleanly.
10. SIGTERM calls `http.Server.Shutdown` with a bounded context.

## Contracts

- Standard library only.
- No retries, rate limiting, or body buffering.
- Request cancellation reaches upstream work.
- Multi-value response cookies remain separate.
- Content-encoding and content-length never describe different bytes.
- Connection-nominated hop-by-hop fields never cross the proxy.
- Upstream errors never expose internal details.

## Test Seams

- Configuration tests isolate environment state.
- Public handler tests use local `httptest` upstreams and downstream servers.
- Private client injection exercises deterministic transport response bodies.
- Streaming tests prove first-chunk delivery, timeout resets, gzip closure, and aborted truncation.
- Process tests cover startup failures and SIGTERM shutdown.

## Remaining Risks

- `go test -race` cannot run on the current Android/arm64 host.
- Shutdown coverage proves idle graceful exit, not completion of an active request.
- Slow request-header timeout needs deployment policy before adding a server-level deadline.
- Invalid startup configuration text includes the supplied base URL and needs an explicit security change to redact it.
