# Tooling and CI

Prefix: TOOL.

- **TOOL1:** Every Make target that runs Go or a scanner runs inside the dev image built from `Dockerfile.dev`, through `DEV_RUN` in the `Makefile` (your UID, `--cap-drop ALL`, `no-new-privileges`, caches under `/tmp`). Do not add a target that needs a host toolchain. CI has Docker, Make, Bash and jq and nothing else.
- **TOOL2:** A tool version changes in one place: `Dockerfile.dev` for Go, Alpine packages, govulncheck and gitleaks; `requirements-semgrep.txt` (hash-locked, every published file hashed) for Semgrep; `go.mod` for golangci-lint. Update the comment in `Dockerfile.dev` and the Go version line in `AGENTS.md` when the Go toolchain moves.
- **TOOL3:** Shell scripts under `scripts/` use the shared `on_error` trap and the JSON `log` helper already in each script.
- **TOOL4:** `.github/workflows/pipeline.yml` runs on every push. Non-tag pushes run `make lint`, `make test-coverage`, `make sec` and `make generate` through `code-workflow.yml`. A `v*` tag skips the checks, waits for that commit's default-branch run through `wait-for-default-branch.yml`, then cuts the GitHub release from the annotated tag. Badges refresh on the default branch and on tags. The weekly schedule exists for the imported-by badge.
- **TOOL5:** `mirror-and-archive.yml`, `issue-pull.yml` and `collaborators-only.yml` are shared across repositories. Change them only on purpose.
