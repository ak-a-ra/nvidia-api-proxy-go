# Design Plan

## Design posture

Port behavior before changing structure. Keep modules aligned with the supplied layout.

## Required sequence

1. Parse and validate configuration.
2. Start a minimal health-only server.
3. Add authentication, routing, and 404 behavior.
4. Add streaming forwarding.
5. Add connect and idle timeouts.
6. Add cancellation and header handling.
7. Add graceful shutdown.
8. Port every original test and compare responses.

## Decisions requiring evidence

- Select the idle-timeout implementation after measuring Go response-controller behavior.
- Confirm base URL path mapping against the original implementation.
- Confirm SIGTERM behavior with an integration test.
