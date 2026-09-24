# e01s01: Parse and validate configuration

## 1. Context
The proxy must reject unusable upstream configuration before listening.

## 2. Problem
Invalid startup configuration must fail deterministically.

## 3. Outcome
A validated configuration supplies defaults and fatal errors.

## 4. Actor
The process operator.

## 5. Scope
Environment parsing, defaults, and NVIDIA_BASE_URL validation.

## 6. Constraints
Go 1.24+ and standard library only.

## 7. Behavior
Missing or invalid NVIDIA_BASE_URL exits with code 1.

## 8. Interface
Configuration is consumed by the server and handler.

## 9. Dependencies
Process environment only.

## 10. Risks
Different URL parsing rules can change startup behavior.

## 11. Assumptions
The original URL validation is authoritative.

## 12. Data
Only environment strings and timeout seconds.

## 13. Security
Do not log credentials or internal URLs.

## 14. Failure Modes
Malformed URLs and unsupported schemes are fatal.

## 15. Acceptance
Startup validation matches server.js cases.

## 16. Test Strategy
Unit-test parsing and subprocess-test exit behavior.

## 17. Gherkin
```gherkin
Scenario: Missing base URL
  Given NVIDIA_BASE_URL is empty
  When the proxy starts
  Then it exits with code 1
```

## 18. Out of Scope
Runtime fallback for invalid base URLs.

## 19. Traceability
Maps to the configuration contract in SCOPE_LATEST.yaml.

## 20. Verification
Run `go test ./... -run TestConfig`.
