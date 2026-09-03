#!/usr/bin/env bash
# gen-builders.sh — generate or verify the per-kind constructor wrappers.
#
#   scripts/gen-builders.sh generate   # rewrite pkg/kubernetes/**/zz_generated_create.go
#                                      # and pkg/kubernetes/zz_generated_kinds_test.go
#   scripts/gen-builders.sh check      # exit 1 when any generated file is stale
#   scripts/gen-builders.sh recover    # delete the generated files, then generate
#
# The generator (pkg/kubernetes/internal/gen) reads the registered scheme, so it
# must be re-run after any change to pkg/kubernetes/scheme.go or to the scope
# table in pkg/kubernetes/internal/kinds. Renovate runs `generate` in its
# postUpgradeTasks after gomod bumps; CI runs `check` in the validate job.
#
# `recover` exists because the generator reaches the scheme through the package
# it writes into: when an API bump removes or renames a kind, the committed
# wrapper still names a Go type that is gone, the kubernetes package no longer
# compiles, and `generate` cannot build the very file that would fix it.
# Deleting the generated files first breaks that cycle -- nothing outside them
# refers to a wrapper -- and `generate` then writes the current set. Use it
# only when `generate` fails to compile; it is not part of CI.
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
  recover)
    find pkg/kubernetes -type f \
      \( -name zz_generated_create.go -o -name zz_generated_create_test.go -o -name zz_generated_kinds_test.go \) \
      -print -delete
    go run ./pkg/kubernetes/internal/gen
    ;;
  *)
    echo "usage: $0 generate|check|recover" >&2
    exit 2
    ;;
esac
