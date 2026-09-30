<!-- story: e01s07 -->
# e01s07: Cancel upstream on client disconnect

## 1. Context
Abandoned client requests must not retain upstream sockets.

## 2. Problem
The proxy needs cancellation before headers and during streaming.

## 3. Outcome
Client disconnect cancels the outbound request in both phases.

## 4. Actor
Disconnected clients and upstream service.

## 5. Scope
Request context propagation and stream cancellation.

## 6. Constraints
No orphaned upstream work and no process crash.

## 7. Behavior
Upstream socket closes after downstream cancellation.

## 8. Interface
Incoming request context to outbound request context.

## 9. Dependencies
Streaming from e01s04.

## 10. Risks
Ignoring cancellation can hold quota and sockets.

## 11. Assumptions
net/http request contexts represent the downstream lifecycle.

## 12. Data
Request cancellation events and socket state.

## 13. Security
Cancellation must not expose upstream details.

## 14. Failure Modes
The proxy remains available after cancellation.

## 15. Acceptance
Pre-header and active-stream cancellation pass.

## 16. Test Strategy
Stub upstream socket tracking with client abort.

## 17. Gherkin
```gherkin
Scenario: Active client disconnect
  Given an upstream stream has emitted one chunk
  When the downstream client cancels
  Then the upstream connection closes
  And the proxy answers a later health request
```

## 18. Out of Scope
Retries and reconnect behavior.

## 19. Traceability
Maps to cancellation requirements in SCOPE_LATEST.yaml.

## 20. Verification
Run `go test ./... -run TestCancellation`.
