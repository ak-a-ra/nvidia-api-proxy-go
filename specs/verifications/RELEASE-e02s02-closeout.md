# Release Handoff: e02s02 closeout (solo-local land)

**Date:** 2026-09-30
**Mode:** `solo-git` (solo-local)
**Decision:** RELEASED — `feat/parity-port` squash-landed on `main`, then a blocking CI failure was fixed via PR before `main` was declared green.

## Supersedes

This record supersedes the `KEEP BRANCH — release integration blocked` decision in
`specs/verifications/RELEASE-e02s02.md` and `specs/verifications/RELEASE-e02s01.md`. Those
records correctly reported that the release gate could not pass *in that environment*. Two
of the three blockers turned out to be provisioning gaps rather than genuine external
prerequisites, and the third was satisfied by wiring CI.

## Blockers from the earlier handoffs, and their resolution

| Recorded blocker | Resolution |
|---|---|
| `scripts/land-branch.sh` absent, so solo-local integration could not run | `scripts/` was never installed in the project. It ships in the installed `bigpowers` package; `bigpowers init` links it into the checkout. Landing ran with the real script. |
| `go test -race` cannot run on `android/arm64` | Wired `.github/workflows/test-build-release.yml` (bundled bigpowers Go template, trimmed) so `-race` runs on `ubuntu-latest`. The race detector is now a standing gate on every push and PR. |
| Blind-spot, completeness-critic, and traceability helper scripts absent | Same `scripts/` provisioning gap. All three now run and are recorded below. |
| No pull request / CI | CI is wired and green on `main`. |

## What was landed

Squash commit `0c58454` on `main` (`feat(proxy): land the behavior-identical Go proxy port`),
followed by `4ffd83b` (`fix(proxy): stop closing the gzip reader during a stream`, PR #1,
squash-merged).

Landed content:

- The behavior-identical port: health and local error shapes, bearer authentication with a
  precomputed SHA-256 digest and constant-time comparison, verbatim `/v1` path and raw query
  forwarding, unbuffered streaming, configured connect and idle timeouts, client disconnect
  cancellation, hop-by-hop header stripping, and SIGTERM graceful shutdown.
- e02 hardening: normalized routing, widened connect-timeout scope, gzip closure reach,
  unsupported-encoding passthrough, connection-nominated header stripping, bodyless framing,
  and `Content-Length` validation.
- Release-gate closeout: per-story verification bundles for `e01s01`–`e01s09`, explicit story
  tags in production and test files, a restored traceability matrix, a recorded gate-trace
  verdict, archived `e01` and `e02` capsules, and the CI workflow.

## Blocking CI failure found after landing

The first push to `main` (`0c58454`) failed the `test` job (`go test ./... -race`, run
36689114367) with a **real data race in production code**, not a flaky test:

```
WARNING: DATA RACE
Read at  ... by goroutine 141:
  compress/flate.(*decompressor).Close()   <- idle-timer goroutine
  compress/gzip.(*Reader).Close()
  (*decodedResponseBody).Close()
Previous write at ... by goroutine 137:
  compress/flate.(*decompressor).nextBlock()   <- request goroutine
  compress/gzip.(*Reader).Read()
  (*decodedResponseBody).Read()
--- FAIL: TestIdleTimeoutClosesTransportBodyForGzipStream
```

`gzip.Reader.Close` is not a resource release: it forwards to the flate decompressor's `Close`,
which reads the decompressor's sticky error field while `Read` writes it. The fix stops calling
it and closes only the upstream source, which is what releases the connection and unblocks an
in-flight `Read`. Full diagnosis: `specs/bugs/BUG-2026-09-30T082134-gzip-close-race.md`.

This is exactly the class of defect the earlier handoffs said could not be checked without a
supported host. Wiring CI found it on the first run.

## Passing checks

Local (`android/arm64`):

- `go test ./... -count=1 && go vet ./... && go build ./...` — PASS
- `go test -count=3 ./...` — PASS
- `go test -count=1 -shuffle=on ./...` — PASS
- `go test -count=5 -run 'TestIdleTimeout|TestCoverage' ./...` — PASS
- Overall statement coverage — **85.0%**, above the 80% gate
- Business-logic coverage (`config.go`, `proxy.go`, `proxy_headers.go`, `proxy_stream.go`) — **100.0%**, above the 95% gate
- Conventional commit subjects — PASS; no AI attribution footer — PASS; secret scan — PASS

CI — every `push` run on `main`:

| Run | Commit | Conclusion |
|---|---|---|
| 36689114367 | `0c58454` | **FAIL** — `test` job caught the gzip data race |
| 36694189176 | `4ffd83b` | PASS — lint, test (`-race`), build |
| 36695587084 | `7e6c669` | PASS — lint, test (`-race`), build |
| 36697276545 | `5bc2209` | PASS — lint, test (`-race`), build |
| 36699423444 | `7e36b39` | PASS — lint, test (`-race`), build |

CI — `pull_request` runs (supporting evidence, not `main` runs):

- 36693908686 — PR #1, all three jobs PASS, the first detector confirmation of the fix
- 36697025485 — PR #3, all three jobs PASS

The first row and the last row are the load-bearing pair: the same `test` job that reported
the race now passes on `main`. Every `push` run on `main` after the fix has passed.

> **Do not freeze a "current tip" in this table.** Each merge to `main` adds a row and makes any
> such marker stale, so the table records history rather than a moving head. Enumerate with
> `gh run list --branch main --event push`; plain `gh run list --branch main` also returns
> `pull_request` runs whose `headBranch` is the feature branch, which is what produced an
> earlier miscitation of a PR run as a `main` run.

Spec gates:

- `check-blind-spots.sh` — 0 HIGH findings (1 MEDIUM + 12 LOW, all vendor-file or permanent-tag noise; see `specs/state.yaml`)
- `scripts/lib/completeness-critic.sh` — BLOCKER=0, WARNING=0
- `scripts/run-gate-trace-verify.sh` — OK
- `golden-g12-status-consistency.sh` — PASS
- gate-trace verdict — PASS, recorded in `specs/execution-status.yaml`

## Residual items

- Overall statement coverage is 85.0%. `main.go` contributes zero statements to the
  parent coverage profile; its startup and SIGTERM paths are covered by subprocess tests.
- `scripts/sync-skills.sh` fails in this checkout (`jq` cannot read the absent
  `package.json`). It is a dev-only maintenance step for the bigpowers package, not part of
  this project's verification stack; `land-branch.sh` therefore ran with `--skip-verify`,
  and the real gate chain was run explicitly and recorded above.
- The race detector runs only on CI. Any change to the streaming, timeout, or shutdown paths
  must rely on a green CI run, not on local green.
- Repository has no tagged release; `release.version` remains null and no publish step exists.

## Evidence

- `specs/bugs/BUG-2026-09-30T082134-gzip-close-race.md`
- `specs/verifications/e02s02-verify.yaml`
- `specs/verifications/e01s01-verify.yaml` … `e01s09-verify.yaml`
- `specs/traceability-matrix.json`, `specs/TRACEABILITY_LATEST.md`
- `specs/state.yaml` (`gate_trace.verdict: PASS`)
- CI: `main` push runs 36689114367 (pre-fix failure) then 36694189176, 36695587084,
  36697276545, 36699423444, all passing; PR runs 36693908686 (PR #1), 36697025485 (PR #3),
  36699184605 (PR #4), and the PR #5 run
