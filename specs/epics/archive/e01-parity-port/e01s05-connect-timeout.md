<!-- story: e01s05 -->
# e01s05: Enforce connect timeout

## 1. Context
The first-response phase needs a bounded wait.

## 2. Problem
A silent upstream must not hold clients indefinitely.

## 3. Outcome
Configured connect timeouts terminate pending upstream work with 502.

## 4. Actor
Clients waiting for upstream response headers.

## 5. Scope
Environment timeout parsing and pending request cancellation.

## 6. Constraints
Zero disables the timeout; invalid values use defaults.

## 7. Behavior
The timer stops when upstream headers arrive.

## 8. Interface
Outbound request context and client transport.

## 9. Dependencies
Streaming request lifecycle from e01s04.

## 10. Risks
A timer that remains active can kill long streams.

## 11. Assumptions
The source names the window a first-response timeout.

## 12. Data
UPSTREAM_CONNECT_TIMEOUT_SECONDS.

## 13. Security
Failure responses stay generic.

## 14. Failure Modes
A silent upstream returns safe 502.

## 15. Acceptance
Positive and zero timeout cases match source tests.

## 16. Test Strategy
Silent stub upstream with bounded assertions.

## 17. Gherkin
```gherkin
Scenario: Silent upstream
  Given the connect timeout is one second
  When the upstream sends no response headers
  Then the proxy returns 502 after the timeout
```

## 18. Out of Scope
Idle timeout and response body reset behavior.

## 19. Traceability
Maps to timeout requirements in SCOPE_LATEST.yaml.

## 20. Verification
Run `go test ./... -run TestConnectTimeout`.
