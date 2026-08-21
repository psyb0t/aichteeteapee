#!/bin/bash
set -euo pipefail

echo "Linting all Go files..."

# `go fix -diff` prints proposed rewrites without applying them. Any output means
# the tree is not go-fix-clean, so fail and point at the auto-fixer.
out=$(go fix -diff ./... 2>&1)
if [ -n "$out" ]; then
	echo "$out"
	echo "go fix found issues. Run 'make lint-fix' to apply."
	exit 1
fi

go tool golangci-lint run --timeout=30m0s ./...
