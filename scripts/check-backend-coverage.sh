#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../backend"

go test ./internal/... -coverprofile=coverage.out >/tmp/cratekeeper-backend-coverage.log
coverage=$(go tool cover -func=coverage.out | awk '/^total:/ { sub(/%/, "", $3); print $3 }')
threshold=90.0

awk -v coverage="$coverage" -v threshold="$threshold" 'BEGIN { exit !(coverage >= threshold) }'

cat /tmp/cratekeeper-backend-coverage.log
go tool cover -func=coverage.out | tail -35
printf 'Backend internal coverage %.1f%% >= %.1f%%\n' "$coverage" "$threshold"
