<!-- story: e01s02 -->
# e01s02: Serve health and local errors

## 1. Context
Health and locally generated errors must remain byte-compatible.

## 2. Problem
The proxy needs a minimal server before upstream forwarding exists.

## 3. Outcome
The service returns exact health and 404 JSON responses.

## 4. Actor
API clients and deployment health checks.

## 5. Scope
Health status, JSON content type, content length, and not-found routing.

## 6. Constraints
No third-party router or framework.

## 7. Behavior
Configured health returns 200; unconfigured health returns 503.

## 8. Interface
HTTP handler boundary.

## 9. Dependencies
Configuration from e01s01.

## 10. Risks
Whitespace and header framing affect byte parity.

## 11. Assumptions
The original JSON serializer produces compact JSON.

## 12. Data
Fixed health and error objects.

## 13. Security
Error bodies contain no internal details.

## 14. Failure Modes
Unknown and bare /v1 paths return 404.

## 15. Acceptance
Local responses match status, body, and content length.

## 16. Test Strategy
Handler-level tests through httptest.

## 17. Gherkin
```gherkin
Scenario: Unconfigured health
  Given the proxy token is missing
  When a client requests /health
  Then the response is 503 with the unconfigured JSON body
```

## 18. Out of Scope
Upstream authentication and forwarding.

## 19. Traceability
Maps to health and 404 requirements in SCOPE_LATEST.yaml.

## 20. Verification
Run `go test ./... -run TestHealth`.
