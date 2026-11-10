---
title: "Development"
description: "Build, lint and test aichteeteapee in its pinned dev image: Make targets, the 90 percent coverage gate, security scans, CI, releases and the docs build."
nav_order: 11
---

# Development

Hacking on aichteeteapee itself. You need Docker and Make, nothing else. Every target runs inside a pinned dev image built from [`Dockerfile.dev`](https://github.com/psyb0t/aichteeteapee/blob/main/Dockerfile.dev) (Go 1.26.9 plus the linter and scanners), so a local run and CI use the exact same toolchain. The container runs as your user with every capability dropped and the checkout mounted at `/work`.

## Make targets

Run `make help` for the list. The [`Makefile`](https://github.com/psyb0t/aichteeteapee/blob/main/Makefile) has:

| Target | What it does |
| --- | --- |
| `make dev-image` | Builds the `aichteeteapee-dev` image. Every other Go target depends on it. |
| `make dep` | `go mod tidy` and `go mod vendor`. |
| `make format` | `gofmt -w` on the Go source, `shfmt -w` on `scripts/*.sh`. |
| `make lint` | [`scripts/lint.sh`](https://github.com/psyb0t/aichteeteapee/blob/main/scripts/lint.sh): shellcheck and `shfmt -d` on the scripts, fails if `go fix -diff ./...` suggests anything, then golangci-lint with the rules in [`.golangci.yml`](https://github.com/psyb0t/aichteeteapee/blob/main/.golangci.yml). |
| `make lint-fix` | Applies `go fix ./...` and runs golangci-lint with `--fix`. |
| `make test` | `go test -race -count=1 -timeout=10m ./...`. |
| `make test-unit` | Same as `make test`. |
| `make test-coverage` | [`scripts/test-coverage.sh`](https://github.com/psyb0t/aichteeteapee/blob/main/scripts/test-coverage.sh): race tests with a coverage profile, writes the total to `coverage-percent.txt`, fails below 90 percent. |
| `make sec` | [`scripts/sec.sh`](https://github.com/psyb0t/aichteeteapee/blob/main/scripts/sec.sh): gitleaks over the working tree and the Git history, govulncheck, and Semgrep (`p/golang` and `p/security-audit`). Writes the merged report to `sec.sarif` and fails on any finding. |
| `make generate` | `go generate ./...`. Nothing generates code today; CI runs it so a future generator cannot drift. |
| `make shell` | A shell in the dev image, for running one test by hand. |
| `make docs` | Builds this site into `.testing/docs-site` with GitHub's own Pages image. |
| `make all` | `dep`, `lint` and `test`. |

[`pre-commit.sh`](https://github.com/psyb0t/aichteeteapee/blob/main/pre-commit.sh) runs `make lint && make test-coverage`, the same pair CI runs.

To run one test:

```bash
make shell
go test -race -run TestBearerAuth ./serbewr/middleware/
```

## Tooling versions

Go, the Alpine packages, govulncheck and gitleaks are pinned in `Dockerfile.dev`, the base image by digest and gitleaks by SHA-256. Semgrep and its dependencies are pinned with hashes in [`requirements-semgrep.txt`](https://github.com/psyb0t/aichteeteapee/blob/main/requirements-semgrep.txt). golangci-lint is a `tool` dependency in [`go.mod`](https://github.com/psyb0t/aichteeteapee/blob/main/go.mod) and runs from the vendored module tree in [`vendor/`](https://github.com/psyb0t/aichteeteapee/tree/main/vendor).

The linter config is strict on purpose: 80-column lines, wrapped errors, no magic numbers, no globals without a reason. A `//nolint` names its linter and says why.

## Tests

Tests sit next to the code in the same package and use testify. The ones that start a real server, hub or socket are the `*_integration_test.go` files; they run in the normal suite. Static file and upload tests read fixtures from [`serbewr/.fixtures/`](https://github.com/psyb0t/aichteeteapee/tree/main/serbewr/.fixtures). Everything runs with the race detector.

## CI

[`.github/workflows/pipeline.yml`](https://github.com/psyb0t/aichteeteapee/blob/main/.github/workflows/pipeline.yml) runs on every push, on any branch, through the shared workflows in [psyb0t/reusable-github-workflows](https://github.com/psyb0t/reusable-github-workflows).

- A branch push runs `make lint`, `make test-coverage`, `make sec` and `make generate`. The runner has Docker and Make; the targets do the rest in the dev image. The `sec.sarif` report goes to the repository's code scanning tab.
- On the default branch, a passing run refreshes the coverage, version, license and imported-by badges on the `badges` branch.
- A `v*` tag does not rerun the checks. It waits for the default-branch run of the same commit to pass, then cuts the GitHub release from the annotated tag message and refreshes the badges with that run's coverage. Push the commit to `main` first, then the tag.
- A weekly scheduled run keeps the imported-by badge current.

## Releases

Releases are annotated `v*` tags on a commit that is already on `main`, and the tag message becomes the release notes. The version is not written into any file. [`CHANGELOG.md`](https://github.com/psyb0t/aichteeteapee/blob/main/CHANGELOG.md) gets an entry per release, and a change that breaks callers starts its line with `Breaking:`.

## Documentation

This site is the [`docs/`](https://github.com/psyb0t/aichteeteapee/tree/main/docs) directory, built by GitHub Pages from `main` with Jekyll and the just-the-docs theme. Every page needs front matter with a unique `title` and a `description` of 50 to 170 characters. Links between pages use the relative `.md` path; a link to anything outside `docs/` uses the full GitHub URL. Code blocks with doubled opening braces, which Go composite literals often have, need the Liquid `raw` tags around them, or Jekyll tries to run them as templates. `make docs` builds the site the way Pages does, into a gitignored directory.
