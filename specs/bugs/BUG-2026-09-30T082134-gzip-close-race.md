# BUG-2026-09-30T082134: gzip response close races with in-flight read

## Problem

- **Actual behavior**: `go test ./... -race` fails on CI (run 36689114367, commit `0c58454`) with a reported data race in the gzip decode path:

  ```
  WARNING: DATA RACE
  Read at 0x00c00042d0e8 by goroutine 141:
    compress/flate.(*decompressor).Close()
      .../src/compress/flate/inflate.go:356
    compress/gzip.(*Reader).Close()
      .../src/compress/gzip/gunzip.go:290
    nvidia-api-proxy-go.(*decodedResponseBody).Close.func1()  proxy_stream.go:42
    sync.(*Once).doSlow() / sync.(*Once).Do()
    nvidia-api-proxy-go.(*decodedResponseBody).Close()          proxy_stream.go:41
    nvidia-api-proxy-go.copyStreaming.func1()                   proxy_stream.go:87

  Previous write at 0x00c00042d0e8 by goroutine 137:
    compress/flate.(*decompressor).nextBlock()                  inflate.go:304
    compress/flate.(*decompressor).Read()                       inflate.go:348
    compress/gzip.(*Reader).Read()                              gunzip.go:252
    nvidia-api-proxy-go.(*decodedResponseBody).Read()           proxy_stream.go:37
    nvidia-api-proxy-go.copyStreaming()                         proxy_stream.go:94

  --- FAIL: TestIdleTimeoutClosesTransportBodyForGzipStream
  ```

- **Expected behavior**: An idle timeout that fires while a gzip-encoded response is being streamed must terminate the stream without an unsynchronized read and write on shared decompressor state. `go test ./... -race` must pass.
- **How to reproduce**: `GOOS=linux GOARCH=amd64 go test -race ./...`, or the CI `test` job. The race needs two goroutines: the request goroutine blocked inside a source `Read`, and the idle-timer goroutine calling `Close`.

## Root Cause Analysis

- **Code path**: A gzip-encoded upstream response is wrapped in a decoding body that owns both a `gzip.Reader` (filtering) and the upstream transport body (source). Streaming copies from the decoding body in the request goroutine. When the idle timeout expires, a timer goroutine calls the decoding body's `Close`.
- **Why it fails**: The decoding body's `Close` performed two steps — close the gzip reader, then close the source. `gzip.Reader.Close` is not a resource release; it delegates to the flate decompressor's `Close`, which *reads* the decompressor's sticky `err` field. `gzip.Reader.Read` *writes* that same field from inside `nextBlock`/`Read`. Nothing serialized those two accesses, so the timer goroutine read the field while the request goroutine wrote it.
- **Contributing factor**: The wrapping body was introduced to reach the transport body on idle close. The upstream `net/http` transport body is already safe for concurrent `Read` and `Close` (it guards both with an internal mutex), so the race exists only in the wrapper added on top of it, not in the transport.
- **Fix direction**: `gzip.Reader.Close` only returns an error and never releases the source (documented: it does not close the underlying reader). The return value was already discarded, so calling it produced no benefit and introduced the only unsynchronized access. Closing the source alone is both sufficient and sufficient to unblock an in-flight read.
- **Risk level: Low**. Removing one discarded-error call removes the shared state without changing the observable stream or close contract. No production behavior, endpoint, or error shape changes.
- **Security impact**: NONE. No security exploit path identified. The race is a single-field read/write on a compression filter inside one request's handler goroutine and its own idle timer; it does not cross request boundaries, so it cannot leak another request's data or bypass authentication. Worst case is a rare race-detector report, or undefined read of a sticky error value.

## TDD Fix Plan

1. **RED**: `go test -race` on the gzip idle-close path. The existing `TestIdleTimeoutClosesTransportBodyForGzipStream` already drives it and fails under `-race` with the stack above. The android/arm64 development host cannot run `-race`, so the reproduction is the CI `test` job; local runs of the same test without `-race` pass and are not evidence.
   **GREEN**: In the decoding body, stop calling the gzip reader's `Close` and close only the source, keeping the single-fire guard. Document why the gzip reader is intentionally left unclosed.
   **verify**: `go test ./... && go vet ./... && go build ./...`, then CI `go test ./... -race`.

2. **RED**: A test that the decoding body closes its source exactly once when read and close overlap.
   **GREEN**: Keep the single-fire close guard.
   **verify**: `go test -count=1 -run 'TestIdleTimeout|TestCoverageStream' ./...`

**REFACTOR**: none required. The decoding body keeps one guard and one close target.

## Acceptance Criteria

- [ ] `go test ./... -race` passes in CI.
- [ ] `TestIdleTimeoutClosesTransportBodyForGzipStream` still asserts the transport body is closed and the stream errors.
- [ ] Gzip response decoding, framing, and header behavior are unchanged.
- [ ] All new tests pass.
- [ ] Existing tests still pass.

## Resolution

<!-- filled in by validate-fix -->
