# Changelog

All notable changes per release. Versions follow [semver](https://semver.org).

## v1.7.1 — 2026-07-27

Self-hosted README badges + `go fix` lint tooling.

- **Coverage / version / license badges** are self-rendered SVGs served from
  `raw.githubusercontent.com/psyb0t/aichteeteapee/badges/*.svg` — no third-party
  render service. `make test-coverage` writes the coverage percentage to
  `coverage-percent.txt`, the pipeline uploads it, and a `badges` job bakes it
  into the SVG. CI status uses GitHub's native badge.
- **Lint tooling:** `make lint` now runs `go fix -diff` as a read-only check (it
  previously applied fixes in-place); run `make lint-fix` to apply. No library
  code changed.

## v1.7.0 and earlier

See the git tags for the pre-CHANGELOG release history — the HTTP library
(`serbewr` router, middleware, WebSocket hubs, file uploads, OpenAPI validation).
