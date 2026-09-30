<!-- story: e01s10 -->
# e01s10: Port and verify all parity tests

## 1. Context
The original suite is the executable parity contract.

## 2. Problem
The Go port needs equivalent observable coverage before release.

## 3. Outcome
All 34 source cases have corresponding Go tests and pass.

## 4. Actor
Maintainers and release reviewers.

## 5. Scope
Behavior-level test port and side-by-side parity review.

## 6. Constraints
No skipped tests and no test dependency additions.

## 7. Behavior
Source and Go tests agree on status, bodies, headers, streams, and lifecycle.

## 8. Interface
Public HTTP handler and subprocess behavior.

## 9. Dependencies
All prior stories.

## 10. Risks
Missing source cases can hide parity drift.

## 11. Assumptions
The 34 tests are complete at the recorded source SHA.

## 12. Data
Stub upstreams, request bodies, headers, and process exits.

## 13. Security
Tests must not use real NVIDIA credentials or internal endpoints.

## 14. Failure Modes
Byte comparisons must account for framework header normalization.

## 15. Acceptance
Every source test maps to one or more Go tests.

## 16. Test Strategy
Use the source suite as a checklist and run go test ./....

## 17. Gherkin
```gherkin
Scenario: Full parity suite
  Given the Go proxy and all ported tests
  When the complete test suite runs
  Then every parity test passes without skipped cases
```

## 18. Out of Scope
New behavior not present in server.test.js.

## 19. Traceability
Covers the full behavior contract in SCOPE_LATEST.yaml.

## 20. Verification
Run `go test ./... -count=1`.
