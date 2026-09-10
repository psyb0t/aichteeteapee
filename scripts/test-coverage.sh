#!/bin/bash
set -euo pipefail
trap 'on_error "$?" "$LINENO"' ERR

LOG_FILE="${LOG_FILE:-/tmp/$(basename "$0" .sh).log}"
exec > >(tee -a "$LOG_FILE") 2>&1

on_error() {
	local exit_code="$1"
	local line="$2"

	printf '{"level":"ERROR","file":"%s","line":%d,"msg":"command failed","exit_code":%d}\n' \
		"${BASH_SOURCE[1]}" "$line" "$exit_code" >&2
}

log() {
	local level="$1"
	shift

	jq -nc \
		--arg time "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
		--arg level "$level" \
		--arg file "${BASH_SOURCE[1]##*/}" \
		--arg line "${BASH_LINENO[0]}" \
		--arg func "${FUNCNAME[1]:-main}" \
		--arg msg "$*" \
		'{time:$time,level:$level,file:$file,line:($line|tonumber),func:$func,msg:$msg}' >&2
}

MIN_TEST_COVERAGE="${MIN_TEST_COVERAGE:-90}"

log INFO "running tests with coverage check"
trap 'rm -f coverage.txt' EXIT
go test -race -coverprofile=coverage.txt ./...

# `go tool cover -func` ends with a "total:\t(statements)\t<pct>%" line. Parse it
# with awk/tr (busybox has no `grep -P`) so this runs in the alpine dev image.
pct=$(go tool cover -func=coverage.txt | awk '/^total:/ {print $3}' | tr -d '%')
if [ -z "$pct" ]; then
	pct=0
fi
printf '%s\n' "$pct" >coverage-percent.txt

int=${pct%%.*}
if [ "$int" -eq 0 ]; then
	log WARN "no test coverage information available"
	exit 0
fi
if [ "$int" -lt "$MIN_TEST_COVERAGE" ]; then
	log ERROR "coverage ${pct}% is below minimum ${MIN_TEST_COVERAGE}%"
	exit 1
fi
log INFO "coverage ${pct}% meets minimum ${MIN_TEST_COVERAGE}%"
