# nvidia-api-proxy-go — AI Agents

> **Multi-agent context** — This file is the canonical project context for Cline, Aider, OpenCode, and other AGENTS.md-native tools. Claude Code and Cursor read it through `CLAUDE.md`.

Read `CONVENTIONS.md` before any GitHub or git operation.

<!-- BEGIN bigpowers:context-routing -->
## Context Routing

| Scope | Context |
|---|---|
| `**/*.go` | `AGENTS.md` and `CONVENTIONS.md` |
| `specs/**` | `AGENTS.md`, `CONVENTIONS.md`, and relevant spec artifact |
| `render.yaml` | `AGENTS.md`, `CONVENTIONS.md`, and deployment notes |
<!-- END bigpowers:context-routing -->

<!-- BEGIN bigpowers:learned-preferences -->
## Learned User Preferences

- (none yet — updated via `session-state`)

## Workspace Facts

- Project path: `nvidia-api-proxy-go/`
- Source contract: behavior-identical port of `nvidia-api-proxy`
- Stack: Go 1.24+ with standard library only
<!-- END bigpowers:learned-preferences -->

<!-- BEGIN bigpowers:project -->
## Project

Behavior-identical Go rewrite of `nvidia-api-proxy`, preserving endpoints, environment variables, error shapes, streaming, timeouts, cancellation, headers, and shutdown behavior.

Stack: Go 1.24+ with `net/http` and other standard-library packages only.

## Commands

| Action | Command |
|---|---|
| Run | `go run .` |
| Test | `go test ./...` |
| Build | `go build ./...` |
| Lint | `go vet ./...` |
| Preflight | `go test ./... && go vet ./... && go build ./...` |
| CI | `gh pr checks` when a pull request exists |

## Test

`go test ./...`

## Lint

`go vet ./...`

## Build

`go build ./...`

## Architecture

`config.go` parses and validates environment variables. `main.go` owns HTTP server setup, signal handling, and graceful shutdown. `proxy.go` owns authentication, routing, header handling, and upstream forwarding. `proxy_test.go` mirrors the original 34 server tests.

## Conventions

- Use Go naming, package-level files, tests beside implementation, and `gofmt`.
- Preserve the original HTTP behavior before optimizing or redesigning.
- Keep files focused and functions small.
- Use standard-library packages only unless requirements change explicitly.
- Keep every implementation task traceable to a spec story.

## Never

- Never add frameworks or third-party dependencies to the parity port.
- Never change behavior during the parity port without an approved requirement.
- Never log API keys, proxy tokens, internal URLs, or DNS details.
- Never use `Header.Get` for multi-value `Set-Cookie` headers.
- Never proxy bare `/v1` or `/v1/` paths.
- Never add retries or rate limiting without explicit opt-in requirements.
- Never dismiss reproducible gate failures as pre-existing or out of scope.
- Never proceed on red Preflight or red CI.

## Agent Rules

- **Workflow Mandate:** Use bigpowers skills such as `plan-work` and `develop-tdd` for structured work.
- **Always Green:** Keep Preflight and CI green before forward work.
- Read `specs/` and `CONVENTIONS.md` before writing code.
- Read `LOG.md` at start. At end of each task, append 3 lines: done, decided, next.
- Write the minimum code that satisfies the requirement.
- Run tests after every change and show evidence before declaring completion.
- Put all planning output in `specs/`.
- Ask one clarifying question before baking uncertain behavior into code.

## Token Economy — Minimal Footprint

1. **Check existing dependencies first.** Inspect current behavior before adding code or packages.
2. **Prefer standard-library primitives.** Use existing Go behavior before writing custom helpers.
3. **Copy validated patterns.** Compare tests and responses against the original implementation.
4. **Keep the simplest working implementation.** Remove unused abstractions and configuration.

## Behavior Contract

- `GET /health` returns configured `200` or unconfigured `503`.
- `/v1/*` requires bearer authentication and forwards paths verbatim.
- Bare `/v1`, `/v1/`, and unknown paths return the exact JSON 404 shape.
- Streaming responses pass through without buffering.
- Upstream failures return `502` without internal details.
- SIGTERM triggers graceful shutdown.
<!-- END bigpowers:project -->
