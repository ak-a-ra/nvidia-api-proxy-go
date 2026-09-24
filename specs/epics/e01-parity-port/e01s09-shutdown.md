# e01s09: Shut down on SIGTERM

## 1. Context
Deployment platforms terminate services with SIGTERM.

## 2. Problem
Shutdown must stop acceptance while allowing active work to finish.

## 3. Outcome
SIGTERM triggers bounded graceful shutdown.

## 4. Actor
Render deployment and active clients.

## 5. Scope
Signal handling, server shutdown, and idle connection closure.

## 6. Constraints
Grace period is bounded and process remains testable.

## 7. Behavior
A SIGTERM service exits successfully after shutdown.

## 8. Interface
OS signal and http.Server lifecycle.

## 9. Dependencies
Server construction from e01s02.

## 10. Risks
Unbounded shutdown can delay deployments.

## 11. Assumptions
The source uses a ten-second forced-exit ceiling.

## 12. Data
Signal and shutdown context.

## 13. Security
No credentials are emitted during shutdown.

## 14. Failure Modes
A stuck stream cannot hold shutdown forever.

## 15. Acceptance
SIGTERM exit code is zero within the source bound.

## 16. Test Strategy
Subprocess signal test with an idle keep-alive request.

## 17. Gherkin
```gherkin
Scenario: SIGTERM shutdown
  Given the proxy is running
  When the process receives SIGTERM
  Then it stops accepting connections and exits with code 0
```

## 18. Out of Scope
Zero-downtime guarantees beyond the original implementation.

## 19. Traceability
Maps to shutdown requirements in SCOPE_LATEST.yaml.

## 20. Verification
Run `go test ./... -run TestShutdown`.
