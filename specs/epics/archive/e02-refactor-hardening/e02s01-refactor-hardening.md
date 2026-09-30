<!-- story: e02s01 -->
# e02s01: Refactor and harden the proxy

## Problem

The completed parity port is structurally small but concentrates routing, transport, compression, header, and streaming policy in one closure. Confirmed edge defects remain in path normalization, timeout scope, gzip closure, and header framing.

## Requirements

### ADDED: Internal proxy handler state

The handler owns one immutable configuration snapshot, one shared HTTP client, and one precomputed token digest.

### MODIFIED: Routing and forwarding

**Before:** Routing accepts `/v1/*` while forwarding the original request URI without canonicalizing dot segments.

**After:** One normalized escaped path controls both routing and forwarding while ordinary paths and raw queries remain unchanged.

### MODIFIED: Connect timeout scope

**Before:** The timeout starts while waiting for response headers after request transmission.

**After:** The timeout covers DNS, connection, request upload, and response-header acquisition, then stops before body streaming.

### MODIFIED: Encoded response handling

**Before:** Every response encoding header is removed, but only gzip bodies are decoded.

**After:** Confirmed gzip responses are decoded without misdeclared framing. Unsupported encodings pass through with their headers and bytes intact.

### MODIFIED: Hop-by-hop headers

**Before:** Only a fixed header deny list is removed.

**After:** Fields nominated by each message's `Connection` header are removed in addition to the fixed list.

### MODIFIED: Idle timeout ownership

**Before:** Closing a gzip decoder can leave the underlying upstream body blocked.

**After:** Idle timeout closure reaches the lowest-level response body.

## Acceptance Criteria

```gherkin
Scenario: Dot segments cannot escape the v1 route
  Given an upstream that records its request URI
  When a caller sends /v1/../admin
  Then the proxy returns the fixed 404 response
  And the upstream is not contacted

Scenario: Upload timeout returns a safe failure
  Given an upstream that does not read request bodies
  And a configured connect timeout
  When a caller streams a request body slowly
  Then the proxy returns the fixed 502 response within the timeout

Scenario: Gzip stall releases the transport body
  Given a gzip response that stops producing bytes
  And a configured idle timeout
  When the idle timeout expires
  Then the upstream request context is canceled

Scenario: Unsupported encoding remains valid
  Given an upstream response with an unsupported content encoding
  When the proxy forwards the response
  Then the content encoding header and original bytes remain intact

Scenario: Full verification stays green
  When the complete verification stack runs
  Then tests, vet, and build pass
```

## Out of Scope

- New endpoints, retries, rate limits, and dependencies.
- Response buffering or package decomposition.
- Unmeasured startup and shutdown redesign.

## Verify

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `go build ./...`
