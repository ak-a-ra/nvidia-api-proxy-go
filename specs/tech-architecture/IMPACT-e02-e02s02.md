# Impact Assessment: e02s02 release coverage

## Target

Test-only coverage for `config.go`, `proxy.go`, `proxy_headers.go`, and `proxy_stream.go`; no production symbol or interface changes.

## Zoom-out

- **Module purpose:** Parse configuration, route authenticated requests, frame upstream responses, and stream response bodies.
- **Callers:** `main.go` constructs `NewHandler`; HTTP server requests enter the handler; local `httptest` servers exercise the public boundary.
- **Contracts:** Exact status/body shapes, normalized routing, safe errors, timeout/cancellation behavior, header framing, and unbuffered streaming remain unchanged.

## Dependents

- `main.go` — constructs the handler and owns process lifecycle; no code change.
- Existing e02 tests — share package-level test helpers; new tests must avoid global state and use channels or synchronous assertions.
- Release gate — consumes `go test -cover ./...` and a weighted business-logic coverage check.

## Affected Stories

- `e02s01` — completed behavior hardening remains the source of truth.
- `e02s02` — new test-only story to close release coverage thresholds.

## Coverage gaps

- `config.go`: invalid `PORT` branches and a few URL/parse boundaries.
- `proxy.go`: empty/root path normalization, missing-token auth, malformed upstream URL, and response-preparation error paths.
- `proxy_headers.go`: malformed/ambiguous content-length boundary.
- `proxy_stream.go`: gzip-construction failure and writer-error boundary.
- `main.go`: process entrypoint is exercised through subprocess tests but contributes zero statements to the parent coverage profile.

## Risk: Low

No production interface or behavior changes. Tests use local upstreams, synthetic credentials, and deterministic transport seams. Risk is limited to test flakiness and timing; tests must use bounded channels/timeouts and run independently.

## Recommended action

Add public-boundary regression tests for the uncovered branches, then rerun coverage, full preflight, audit, independent review, and release gates. Do not add dependencies or production refactors.
