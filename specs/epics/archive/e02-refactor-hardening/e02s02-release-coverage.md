<!-- story: e02s02 -->
# e02s02: Close release coverage gates

## 1. Context

`e02s01` passed behavioral verification, but the release-branch gate reports 76.9% overall statement coverage and an estimated 90.5% business-logic coverage when `main.go` is excluded. The implementation is not being changed; this story adds focused tests for existing public contracts and edge branches.

## 2. Problem

Release readiness requires measurable coverage, and uncovered error and boundary paths can hide regressions in configuration, routing, response framing, and streaming.

## 3. Outcome

Existing behavior is covered at the public HTTP boundary and through deterministic transport seams, with no production dependency or contract changes.

## 4. Actor

Maintainers and release reviewers.

## 5. Scope

- Configuration validation edge cases.
- Routing normalization and authentication boundaries.
- Malformed upstream request/response preparation paths.
- Content-length, gzip initialization, and downstream write-error boundaries.
- Coverage and full-stack verification evidence.

## 6. Constraints

- Go 1.24+ and standard library only.
- No production behavior changes, retries, buffering, new endpoints, or third-party packages.
- Tests use synthetic credentials and local upstreams only.
- Tests remain independent, repeatable, and self-validating.

## 7. Behavior

All new tests assert existing behavior through `NewHandler` or the smallest deterministic I/O seam permitted by `CONVENTIONS.md`.

## 8. Interface

No interface changes. Tests may use `NewHandler`, `httptest`, and package-local deterministic `RoundTripper`/writer seams already used by the suite.

## 9. Dependencies

Completed `e02s01` implementation and its passing regression suite.

## 10. Risks

Timing-based transport tests can flake; bounded deadlines, cleanup, and channels prevent shared-state races.

## 11. Assumptions

The release gate thresholds are 80% overall coverage and 95% business-logic coverage. Business logic means `config.go`, `proxy.go`, `proxy_headers.go`, and `proxy_stream.go`; `main.go` is process glue measured separately by subprocess tests.

## 12. Data

Synthetic API keys, local HTTP servers, malformed URLs, invalid headers, deterministic response bodies, and bounded timeout values.

## 13. Security

Tests must not contact NVIDIA or other external services. Credentials remain synthetic. No new security behavior is introduced.

## 14. Failure Modes

A test must not wait indefinitely for a transport or timer. Cleanup must close local servers, bodies, channels, and subprocesses.

## 15. Acceptance

Overall statement coverage reaches at least 80%; weighted coverage for the four business-logic files reaches at least 95%; all tests, vet, and build pass.

## 16. Test Strategy

Add table-driven boundary tests where possible, local `httptest` servers for HTTP behavior, deterministic injected transports for body-construction errors, and focused commands after each vertical slice.

## 17. Gherkin

```gherkin
Scenario: Configuration boundaries reject invalid ports
  Given a valid NVIDIA base URL
  When PORT contains a non-numeric or out-of-range value
  Then LoadConfig returns the documented port validation error

Scenario: Handler rejects malformed upstream configuration safely
  Given a handler with an invalid upstream origin
  When an authenticated v1 request reaches the handler
  Then the client receives the fixed safe 502 response

Scenario: Response preparation errors remain safe
  Given an upstream response with an invalid gzip body or invalid framing
  When the proxy forwards the response
  Then the handler returns a safe failure or drops invalid framing

Scenario: Release coverage gate passes
  When the complete coverage command runs
  Then overall coverage is at least 80 percent and business-logic coverage is at least 95 percent
  And go test, go vet, and go build pass
```

## 18. Out of Scope

- Production refactoring or behavior changes.
- New `main` package abstractions solely for coverage.
- NFR threshold redesign, server deadlines, active-shutdown redesign, or package decomposition.
- Third-party coverage tooling.

## 19. Traceability

Closes the release-gate gaps recorded in `specs/verifications/RELEASE-e02s01.md` while preserving the e02s01 behavior contract.

## 20. Verification

- `go test -cover ./...`
- `go test -count=1 ./...`
- `go vet ./...`
- `go build ./...`
