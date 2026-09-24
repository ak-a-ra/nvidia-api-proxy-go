# e01s08: Copy headers without transport leakage

## 1. Context
Proxy headers must preserve semantics without forwarding transport metadata.

## 2. Problem
Hop-by-hop headers leak sockets and stale framing.

## 3. Outcome
Transport headers are stripped and cookies retain the source's special handling.

## 4. Actor
Clients and upstream services.

## 5. Scope
Request and response header copying, compression headers, and content length.

## 6. Constraints
Response Set-Cookie values remain separate; request cookie values join.

## 7. Behavior
Content-Encoding is not forwarded when net/http decompresses the body.

## 8. Interface
Outbound and inbound HTTP header maps.

## 9. Dependencies
Streaming from e01s04.

## 10. Risks
Merging response cookies changes observable behavior.

## 11. Assumptions
The source header sets define the exact allow/deny behavior.

## 12. Data
Header names, values, and content lengths.

## 13. Security
Authorization is replaced with the server-side key.

## 14. Failure Modes
Stale content length can corrupt response framing.

## 15. Acceptance
Header stripping and cookie tests match source cases.

## 16. Test Strategy
Stub upstream with multi-value and encoded responses.

## 17. Gherkin
```gherkin
Scenario: Multi-value response cookies
  Given the upstream emits two Set-Cookie values
  When the proxy forwards the response
  Then both values remain separate
```

## 18. Out of Scope
Changing response content negotiation behavior.

## 19. Traceability
Maps to header requirements in SCOPE_LATEST.yaml.

## 20. Verification
Run `go test ./... -run TestHeaders`.
