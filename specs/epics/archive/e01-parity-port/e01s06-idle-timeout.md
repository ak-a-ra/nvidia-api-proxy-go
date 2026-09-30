<!-- story: e01s06 -->
# e01s06: Enforce idle timeout

## 1. Context
Response bodies can remain open while producing no bytes.

## 2. Problem
A silent stream must close without ending active streams that continue.

## 3. Outcome
The idle deadline resets after every received chunk.

## 4. Actor
Clients receiving streaming responses.

## 5. Scope
Per-chunk idle deadline and stream error handling.

## 6. Constraints
Zero disables the deadline; long streams survive repeated chunks.

## 7. Behavior
A stalled response is cut; a slow-drip response completes.

## 8. Interface
Response body reader and streaming copy loop.

## 9. Dependencies
Streaming from e01s04.

## 10. Risks
A fixed deadline incorrectly truncates active streams.

## 11. Assumptions
The source resets the watchdog on each data event.

## 12. Data
UPSTREAM_IDLE_TIMEOUT_SECONDS and body chunks.

## 13. Security
Internal timeout details never enter client JSON.

## 14. Failure Modes
Idle failure destroys only the affected response.

## 15. Acceptance
Stall, disabled, and active-long cases match source tests.

## 16. Test Strategy
Slow-drip stub upstream with chunk arrival assertions.

## 17. Gherkin
```gherkin
Scenario: Stalled stream
  Given the upstream emits one chunk and then stops
  When the idle timeout expires
  Then the downstream stream fails
  And the proxy remains healthy
```

## 18. Out of Scope
Connect timeout and client cancellation.

## 19. Traceability
Maps to idle timeout requirements in SCOPE_LATEST.yaml.

## 20. Verification
Run `go test ./... -run TestIdleTimeout`.
