# nvidia-api-proxy-go — Conventions

## Project

`nvidia-api-proxy-go` is a behavior-identical Go rewrite of `nvidia-api-proxy`.

- Preserve endpoints, environment variables, error shapes, streaming, and edge cases.
- Treat the original `server.js` and its tests as the behavior authority.
- Keep parity work separate from optimization and redesign.

## Commit Messages

All commits MUST follow Conventional Commits 1.0.0.

- Format: `type(scope): description`.
- Put one space after the colon.
- Use `feat`, `fix`, `perf`, `docs`, `chore`, `style`, `refactor`, or `test`.
- Use `BREAKING CHANGE:` for incompatible changes.
- Do not add AI attribution footers.

## Workflow

- MUST use bigpowers skills for structured work.
- MUST read `specs/` and this file before writing code.
- MUST create a feature branch or worktree after repository initialization.
- MUST add runnable `verify:` commands to every implementation task.
- MUST run Preflight before advancing workflow phases.
- MUST record discoveries and handoffs in `specs/`.

## Always Green / Shift Left

Always Green means the complete local verification stack and CI stay green.

- MUST treat red Preflight as a blocker.
- MUST treat red CI as a blocker.
- MUST use the 1-10-100 principle: fix defects early because later fixes cost more.
- MUST run `go test ./... && go vet ./... && go build ./...` before handoff.
- MUST rerun the failing command after every fix.

## Discovered Defects

Use this ladder for every reproducible gate failure.

1. Use `quick-fix` for trivial, data-only, or single-file defects.
2. Use `fix-bug` when root cause needs investigation or TDD.
3. Write a BUG spec and stop forward work when reproduction remains blocked.

Ship discovered fixes separately from unrelated feature work.

## Planning Output

All planning and investigation output belongs under `specs/`.

- Product intent: `specs/product/`
- Architecture: `specs/tech-architecture/`
- Epics and stories: `specs/epics/`
- Bugs: `specs/bugs/`
- Verification evidence: `specs/verifications/`
- Session state: `specs/state.yaml`

## Go Stack Conventions

- MUST target Go 1.24 or newer.
- MUST use `gofmt` for formatting.
- MUST use `go vet ./...` for static checks.
- MUST use `net/http` and standard-library packages only by default.
- MUST pass request contexts into upstream requests.
- MUST preserve `Set-Cookie` values with `Header.Values`.
- MUST strip hop-by-hop headers in both directions.
- MUST keep functions focused and files below 300 lines.
- MUST name constants instead of embedding magic values.
- MUST test observable HTTP behavior through public handlers.

## Defensive Code

- MUST implement configured upstream connect and idle timeouts.
- MUST preserve zero timeout values as disabled.
- MUST cancel upstream work when clients disconnect.
- MUST return safe `502` responses when upstream work fails.
- MUST support graceful SIGTERM shutdown.
- MUST NOT add retries or rate limiting without explicit opt-in requirements.

## Tests

Tests MUST be Fast, Independent, Repeatable, Self-Validating, and Timely.

- Port all 34 original `server.test.js` cases.
- Add explicit slow-drip idle-timeout coverage.
- Add client-disconnect cancellation coverage.
- Add boundary coverage for zero and positive timeout values.
- Run tests through `go test ./...`.
- Never skip tests without a documented ambiguity note.

## Never

- Never add a web framework or third-party dependency without approval.
- Never change the behavior contract during the parity port.
- Never expose API keys, proxy tokens, internal URLs, or DNS errors.
- Never use `Header.Get` for multi-value `Set-Cookie` headers.
- Never proxy bare `/v1` or `/v1/`.
- Never use a single read deadline for per-chunk idle timeout resets.
- Never add retries or rate limiting by default.
- Never dismiss reproducible gate failures as pre-existing, unrelated, or out of scope.
- Never proceed on red Preflight or red CI.

## Tool Wiring

- `CLAUDE.md`, `GEMINI.md`, and `AGENTS.md` share this project context.
- `opencode.json` loads `AGENTS.md` and bigpowers Cursor rules.
- `.aider.conf.yml` reads `AGENTS.md`.
- `.codex/config.toml` loads `AGENTS.md`.
- `.cursor/rules` links the installed bigpowers rules.
