# Security Plan

## Secrets

- Read `NVIDIA_API_KEY` and `PROXY_AUTH_TOKEN` from environment variables.
- Never log or return secret values.
- Use constant-time bearer-token comparison.

## Network boundaries

- Strip hop-by-hop headers in both directions.
- Preserve `Set-Cookie` values without logging them.
- Return `502 {"error":"Bad gateway"}` for upstream failures.
- Never expose internal URLs, DNS errors, or connection details.

## Startup

- Reject missing or invalid `NVIDIA_BASE_URL` with exit code 1.
- Keep timeout defaults explicit and disable them only when configured as zero.
