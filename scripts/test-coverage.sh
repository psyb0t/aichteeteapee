#!/bin/bash
set -euo pipefail

MIN_TEST_COVERAGE="${MIN_TEST_COVERAGE:-85}"

echo "Running tests with coverage check..."
trap 'rm -f coverage.txt' EXIT
go test -race -coverprofile=coverage.txt ./...

# `go tool cover -func` ends with a "total:\t(statements)\t<pct>%" line. Parse it
# with awk/tr (busybox has no `grep -P`) so this runs in the alpine dev image.
pct=$(go tool cover -func=coverage.txt | awk '/^total:/ {print $3}' | tr -d '%')
[ -n "$pct" ] || pct=0
printf '%s\n' "$pct" >coverage-percent.txt

int=${pct%%.*}
if [ "$int" -eq 0 ]; then
	echo "No test coverage information available."
	exit 0
fi
if [ "$int" -lt "$MIN_TEST_COVERAGE" ]; then
	echo "FAIL: Coverage ${pct}% is less than the minimum ${MIN_TEST_COVERAGE}%"
	exit 1
fi
echo "Coverage ${pct}% meets the minimum requirement of ${MIN_TEST_COVERAGE}%"
