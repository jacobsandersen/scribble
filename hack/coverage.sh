#!/usr/bin/env bash
set -euo pipefail

# Run Go tests with coverage and summarize results.
# Optional: set COVERAGE_THRESHOLD (e.g., 80) to enforce a minimum percentage.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

COVER_PROFILE="coverage.out"
COVER_FUNC="coverage.txt"

# Clean previous artifacts
rm -f "$COVER_PROFILE" "$COVER_FUNC"

echo "==> computing package list"
PKGS=$(go list ./...)

echo "Packages under coverage:" $PKGS

# Include testcontainers by default; allow opt-out locally via SKIP_TESTCONTAINERS=1
GO_TEST_TAGS="-tags=testcontainers"
if [[ "${SKIP_TESTCONTAINERS:-}" == "1" ]]; then
  echo "SKIP_TESTCONTAINERS=1 set; running without testcontainers tag"
  GO_TEST_TAGS=""
fi

echo "==> go test with coverage (${GO_TEST_TAGS:-no tags})"
# Use per-package coverage (no cross-package -coverpkg) to match `go test ./... -coverprofile` output.
go test ${GO_TEST_TAGS} ./... -coverprofile="$COVER_PROFILE" -covermode=atomic

echo "==> coverage summary"
go tool cover -func="$COVER_PROFILE" | tee "$COVER_FUNC"

TOTAL_LINE=$(grep "^total:" "$COVER_FUNC" || true)
if [[ -z "$TOTAL_LINE" ]]; then
  echo "Failed to parse total coverage from $COVER_FUNC" >&2
  exit 1
fi

TOTAL_PCT=$(echo "$TOTAL_LINE" | awk '{print $3}' | tr -d '%')
if [[ -z "$TOTAL_PCT" ]]; then
  echo "Failed to extract coverage percentage" >&2
  exit 1
fi

echo "Total coverage: ${TOTAL_PCT}%"

if [[ -n "${COVERAGE_THRESHOLD:-}" ]]; then
  # Compare as integer; assumes threshold provided as whole number (e.g., 80)
  THRESH=${COVERAGE_THRESHOLD%.*}
  PCT_INT=${TOTAL_PCT%.*}
  if (( PCT_INT < THRESH )); then
    echo "Coverage ${TOTAL_PCT}% is below threshold ${THRESH}%" >&2
    exit 2
  fi
  echo "Coverage meets threshold ${THRESH}%"
fi

