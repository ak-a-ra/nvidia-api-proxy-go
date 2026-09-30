<!-- story: e01s03 -->
# e01s03: Authenticate and route requests

## 1. Context
Only authenticated /v1/* requests reach the upstream.

## 2. Problem
The proxy must enforce authentication without revealing missing credentials.

## 3. Outcome
Requests route with exact paths, queries, and auth replacement.

## 4. Actor
Clients using the proxy token.

## 5. Scope
Bearer authentication, path routing, and upstream URL origin mapping.

## 6. Constraints
Constant-time comparison and no base-path duplication.

## 7. Behavior
401 precedes the unconfigured 503 response.

## 8. Interface
Handler and upstream request construction.

## 9. Dependencies
Configuration and local routing from e01s02.

## 10. Risks
Incorrect path joining can change upstream behavior.

## 11. Assumptions
The incoming path and query are forwarded verbatim.

## 12. Data
Authorization header and request URL.

## 13. Security
Hash before constant-time comparison to avoid length timing leaks.

## 14. Failure Modes
Malformed authorization returns 401.

## 15. Acceptance
Path, query, token, and status behavior match source tests.

## 16. Test Strategy
Stub upstream integration tests.

## 17. Gherkin
```gherkin
Scenario: Authenticated route
  Given a valid proxy token
  When a client requests /v1/models?x=1
  Then the upstream receives /v1/models?x=1
  And the upstream receives the NVIDIA API key authorization
```

## 18. Out of Scope
Upstream response streaming and header policy.

## 19. Traceability
Maps to routing and auth requirements in SCOPE_LATEST.yaml.

## 20. Verification
Run `go test ./... -run TestProxyRouting`.
