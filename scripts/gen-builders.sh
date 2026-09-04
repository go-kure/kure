#!/usr/bin/env bash
# gen-builders.sh — generate or verify the per-kind constructor wrappers and the
# kinds/scope/maturity tables.
#
#   scripts/gen-builders.sh generate   # rewrite pkg/kubernetes/**/zz_generated_create.go,
#                                      # pkg/kubernetes/zz_generated_kinds_test.go,
#                                      # pkg/kubernetes/zz_generated_tables.go and
#                                      # docs/api-tables.{json,md}
#   scripts/gen-builders.sh check      # exit 1 when any generated file is stale
#   scripts/gen-builders.sh recover    # delete the wrappers, then generate
#
# The generator (pkg/kubernetes/internal/gen) reads the registered scheme and
# the pinned upstream module sources, so it must be re-run after any change to
# pkg/kubernetes/scheme.go, to pkg/kubernetes/internal/kinds, or to a pinned
# API module. Renovate runs `generate` in its postUpgradeTasks after gomod
# bumps; CI runs `check` in the validate job.
#
# `recover` exists because the generator reaches the scheme through the package
# it writes into: when an API bump removes or renames a kind, the committed
# wrapper still names a Go type that is gone, the kubernetes package no longer
# compiles, and `generate` cannot build the very file that would fix it.
# Deleting the generated files first breaks that cycle -- nothing outside them
# refers to a wrapper -- and `generate` then writes the current set. Use it
# only when `generate` fails to compile; it is not part of CI.
#
# zz_generated_tables.go is deliberately NOT deleted by `recover`. It holds only
# strings and bools -- it never names an upstream Go type, so an API bump cannot
# make it uncompilable -- and pkg/kubernetes/tables.go reads the values it
# declares. Deleting it would break the package rather than repair it, and the
# generator could then no longer run at all. Its absence is a compile error on
# purpose: an empty table would answer "not registered" for every kind, which is
# a wrong answer rather than a missing one. docs/api-tables.{json,md} are not
# Go and are simply overwritten.
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
