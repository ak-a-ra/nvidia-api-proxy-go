# Tech Stack

## Runtime

- Go 1.24+
- Standard library only
- `net/http` server and client
- `render.yaml` deployment target

## Modules

- `config.go`: environment parsing, defaults, validation, and startup failure.
- `main.go`: server construction, signal handling, and graceful shutdown.
- `proxy.go`: authentication, routing, header handling, streaming, and timeouts.
- `proxy_test.go`: parity tests and edge-case coverage.

## Gray Areas

- Per-chunk idle timeout reset requires a custom reader or response-controller loop.
- Client cancellation must reach the upstream request context.
- Multi-value `Set-Cookie` handling requires `Header.Values`.
- Base URL path joining must preserve incoming path mapping.
