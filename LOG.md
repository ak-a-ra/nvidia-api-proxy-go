# LOG

Append one line under each heading at the end of every task.

## done

- 2026-09-30 repo init: Go 1.24+ standard-library port of `nvidia-api-proxy`; parity port landed, CI wired (`go vet`, `go test -race`, build), 85.0% statement coverage with 100% on `config.go`, `proxy.go`, `proxy_headers.go`, `proxy_stream.go`.
- 2026-09-30 nfr-targets branch: `nfr_test.go` added with 8 tests enforcing 4 numeric NFR targets; `specs/tech-architecture/NFR_LATEST.yaml` created as contract source of truth; `TEST_PLAN_LATEST.md`, `e02s01-verify.yaml`, and `state.yaml` updated to close `NFR-E02-001`.

## decided

- 2026-09-30 integration and NFR policy: land solo-locally to `main` via `scripts/land-branch.sh` with no PR ceremony; run `-race` on CI only, because this android/arm64 host cannot run it; and e02 P0 NFRs are stated as numeric targets, not "none apply", per the human decision of 2026-09-30.
- 2026-09-30 NFR measurement policy: allocation figures reported as observed ranges because `sync.Pool` hit/miss causes ~30 KiB variance; body-size independence is the enforced invariant, not exact byte equality across runs.

## next

- Land `feat/nfr-targets` to `main` and confirm CI green.
- Decide whether to cut the first release tag; `release.version` is still null in `specs/release-plan.yaml`.
