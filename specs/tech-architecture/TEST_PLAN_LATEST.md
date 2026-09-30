# Test Plan

## Required coverage

- Port all 34 original `server.test.js` cases.
- Test health configured and unconfigured responses.
- Test bearer authentication, including invalid and missing tokens.
- Test exact 404 and 502 response bodies.
- Test path forwarding with base URLs containing or omitting `/v1`.
- Test streaming chunk boundaries and slow-drip idle timeout resets.
- Test connect timeout, idle timeout, and zero-value disable behavior.
- Test client disconnect cancellation.
- Test hop-by-hop header removal and multi-value `Set-Cookie` preservation.
- Test SIGTERM graceful shutdown.

## Non-functional targets

Source of truth: `specs/tech-architecture/NFR_LATEST.yaml`. Ratified by human decision on
2026-09-30, superseding the open item `NFR-E02-001`. Each target is enforced by a test in
`nfr_test.go`, so a regression fails the suite instead of going unnoticed.

| Id | Threshold | Enforced by |
|---|---|---|
| `NFR-E02-1` | Streaming copy of a 32 MiB body allocates less than 512 KiB, independent of body size | `TestNFRStreamingAllocationIsConstantInBodySize`, `TestNFRStreamingGzipDecodeDoesNotBufferBody` |
| `NFR-E02-2` | Worst first-chunk latency under 32 concurrent gated streams is less than 500 ms | `TestNFRStreamingFirstChunkNotBufferedUnderLoad` |
| `NFR-E02-3` | 256 concurrent streaming requests complete with 0 failures and byte-exact bodies in less than 10 s | `TestNFRStreamingHandlesTwoHundredFiftySixConcurrentRequests` |
| `NFR-E02-4` | Connect timeout T yields the fixed 502 between T and T + 1 s; idle timeout T ends a stall within T + 500 ms; chunk arrival resets the idle bound; `0` disables either bound; defaults are 30 s and 120 s | `TestNFRConnectTimeoutHonorsConfiguredBound`, `TestNFRConnectTimeoutZeroDisablesTheBound`, `TestNFRIdleTimeoutBoundsStallsWithoutCuttingActiveStreams`, `TestNFRTimeoutDefaultsAreNumeric` |

## Commands

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `go build ./...`
- `go test -run 'TestNFR' -v ./...`
