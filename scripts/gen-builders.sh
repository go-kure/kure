#!/usr/bin/env bash
# gen-builders.sh — generate or verify the per-kind constructor wrappers.
#
#   scripts/gen-builders.sh generate   # rewrite pkg/kubernetes/**/zz_generated_create.go
#                                      # and pkg/kubernetes/zz_generated_kinds_test.go
#   scripts/gen-builders.sh check      # exit 1 when any generated file is stale
#
# The generator (pkg/kubernetes/internal/gen) reads the registered scheme, so it
# must be re-run after any change to pkg/kubernetes/scheme.go or to the scope
# table in pkg/kubernetes/internal/kinds. Renovate runs `generate` in its
# postUpgradeTasks after gomod bumps; CI runs `check` in the validate job.
set -euo pipefail

cd "$(dirname "$0")/.."
export GOWORK="${GOWORK:-off}"

case "${1:-}" in
  generate)
    go run ./pkg/kubernetes/internal/gen
    ;;
  check)
    go run ./pkg/kubernetes/internal/gen -check
    ;;
  *)
    echo "usage: $0 generate|check" >&2
    exit 2
    ;;
esac
