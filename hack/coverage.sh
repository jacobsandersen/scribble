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
COVERPKG=$(echo "$PKGS" | paste -sd, -)

echo "Packages under coverage:" $PKGS
echo "==> go test with coverage"
go test $PKGS -coverpkg="$COVERPKG" -coverprofile="$COVER_PROFILE" -covermode=atomic

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

