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

log INFO "linting all Go files"

if ! shellcheck scripts/*.sh; then
	log ERROR "shellcheck failed"
	exit 1
fi

if ! shfmt -d scripts/*.sh; then
	log ERROR "shfmt check failed"
	exit 1
fi

# `go fix -diff` prints proposed rewrites without applying them. Any output means
# the tree is not go-fix-clean, so fail and point at the auto-fixer.
if ! out=$(go fix -diff ./... 2>&1); then
	printf '%s\n' "$out" >&2
	log ERROR "go fix check failed"
	exit 1
fi

if [ -n "$out" ]; then
	printf '%s\n' "$out" >&2
	log ERROR "go fix found issues, run make lint-fix"
	exit 1
fi

if ! go tool golangci-lint run --timeout=30m0s ./...; then
	log ERROR "golangci-lint failed"
	exit 1
fi

log INFO "lint complete"
