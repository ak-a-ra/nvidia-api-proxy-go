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

## Commands

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `go build ./...`
