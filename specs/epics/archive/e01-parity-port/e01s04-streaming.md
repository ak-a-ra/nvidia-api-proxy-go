<!-- story: e01s04 -->
# e01s04: Stream upstream bodies

## 1. Context
The proxy must preserve streaming responses without buffering.

## 2. Problem
Buffered forwarding breaks SSE and long-lived responses.

## 3. Outcome
Request and response bodies are forwarded incrementally.

## 4. Actor
Streaming API clients.

## 5. Scope
Body forwarding, status passthrough, and stream lifecycle.

## 6. Constraints
Use net/http and io streaming primitives.

## 7. Behavior
Chunks reach the client as they arrive from upstream.

## 8. Interface
Outbound request and response body streams.

## 9. Dependencies
Routing and authentication from e01s03.

## 10. Risks
Intermediate buffering changes delivery timing.

## 11. Assumptions
The original SSE test is the minimum streaming contract.

## 12. Data
Request and response byte streams.

## 13. Security
Do not log body contents or credentials.

## 14. Failure Modes
Mid-stream errors must not crash the process.

## 15. Acceptance
SSE chunks and ordinary response bytes match.

## 16. Test Strategy
Stub upstream with immediate and delayed chunks.

## 17. Gherkin
```gherkin
Scenario: SSE passthrough
  Given the upstream emits two SSE chunks
  When an authenticated client requests the route
  Then both chunks arrive without response buffering
```

## 18. Out of Scope
Idle timeout and cancellation handling.

## 19. Traceability
Maps to streaming requirements in SCOPE_LATEST.yaml.

## 20. Verification
Run `go test ./... -run TestStreaming`.
