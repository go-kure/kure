#!/bin/sh
# check-consumer.sh - check that a module which imports kure compiles, and that
# its module graph holds no v7-era kube-openapi (go-kure/kure#1316).
#
# Why: Go applies `replace` directives only in the main module. kure's own build
# honours the replaces in its go.mod, and a module that requires kure ignores
# them and reads only kure's `require` lines. A graph that kure's own build
# never sees can therefore pull a kube-openapi commit that needs
# sigs.k8s.io/structured-merge-diff/v7, while apimachinery v0.37 uses v6, and the
# consumer fails to compile. This script builds that consumer view.
#
# It makes a throwaway module in a temporary directory. The module requires kure
# through a `replace` to the kure tree under test, imports kure's packages, and
# has none of kure's replaces. Then it checks two things:
#   1. the module graph holds no sigs.k8s.io/structured-merge-diff/v7;
#   2. the selected k8s.io/kube-openapi is the version kure's go.mod requires.
# It also builds the module. It prints every result and fails if any check fails.
#
# Remove this script with the Kubernetes v0.38 GA bump, whose apimachinery moves
# to v7 (docs/dependency-updates.md).
#
# Usage: scripts/check-consumer.sh [KURE_DIR]
#   KURE_DIR  the kure tree to test (default: the tree this script is in).
# Needs `go` and `awk` on PATH, and network or a warm module cache.
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
KURE_DIR="$(cd "${1:-$ROOT}" && pwd)" || exit 2
KURE_MODULE="github.com/go-kure/kure"
OPENAPI="k8s.io/kube-openapi"
SMD7="sigs.k8s.io/structured-merge-diff/v7"

# A workspace file above the temporary module would change the graph.
GOWORK=off
export GOWORK
# The temporary module has no go.sum. -mod=mod lets tidy write one.
GOFLAGS=-mod=mod
export GOFLAGS

if [ ! -f "$KURE_DIR/go.mod" ]; then
    echo "check-consumer: no go.mod in $KURE_DIR" >&2
    exit 2
fi

WORK="$(mktemp -d "${TMPDIR:-/tmp}/check-consumer.XXXXXX")" || exit 2
trap 'rm -rf "$WORK"' EXIT INT TERM

GO_DIRECTIVE="$(awk '$1 == "go" {print $2; exit}' "$KURE_DIR/go.mod")"
if [ -z "$GO_DIRECTIVE" ]; then
    echo "check-consumer: no go directive in $KURE_DIR/go.mod" >&2
    exit 2
fi

# The require line of kube-openapi in kure's go.mod. A replace line has "=>" in
# its second field, so a version in the second field is the require line.
KURE_REQUIRES="$(awk -v m="$OPENAPI" '$1 == m && $2 ~ /^v/ {print $2; exit}' "$KURE_DIR/go.mod")"
if [ -z "$KURE_REQUIRES" ]; then
    echo "check-consumer: $KURE_DIR/go.mod has no require line for $OPENAPI" >&2
    exit 2
fi

cat > "$WORK/go.mod" <<EOF
module example.com/kure-consumer

go $GO_DIRECTIVE

require $KURE_MODULE v0.0.0

replace $KURE_MODULE => "$KURE_DIR"
EOF

cat > "$WORK/main.go" <<'EOF'
package main

import (
	_ "github.com/go-kure/kure/pkg/kubernetes"
	_ "github.com/go-kure/kure/pkg/stack"
)

func main() {}
EOF

cd "$WORK" || exit 2
fail=0

echo "check-consumer: kure tree $KURE_DIR"
echo "check-consumer: kure requires $OPENAPI $KURE_REQUIRES"

if ! go mod tidy 2>"$WORK/tidy.err"; then
    echo "FAIL: go mod tidy in the consumer module:"
    sed 's/^/    /' "$WORK/tidy.err"
    exit 1
fi

if go mod graph > "$WORK/graph.txt" 2>"$WORK/graph.err"; then
    smd7_count="$(grep -c " $SMD7@\|^$SMD7@" "$WORK/graph.txt" || true)"
    if [ "$smd7_count" = "0" ]; then
        echo "PASS: no $SMD7 in the consumer module graph"
    else
        echo "FAIL: $SMD7 is in the consumer module graph, $smd7_count edge(s); the modules that require it:"
        grep " $SMD7@" "$WORK/graph.txt" | awk '{print "    " $1}' | sort -u
        fail=1
    fi
else
    echo "FAIL: go mod graph in the consumer module:"
    sed 's/^/    /' "$WORK/graph.err"
    fail=1
fi

selected="$(go list -m -f '{{.Version}}' "$OPENAPI" 2>/dev/null || true)"
echo "check-consumer: selected $OPENAPI $selected"
if [ "$selected" = "$KURE_REQUIRES" ]; then
    echo "PASS: the selected $OPENAPI is the one kure requires"
else
    echo "FAIL: the selected $OPENAPI is '$selected', kure requires $KURE_REQUIRES"
    fail=1
fi

if go build ./... 2>"$WORK/build.err"; then
    echo "PASS: the consumer module builds"
else
    echo "FAIL: the consumer module does not build:"
    head -n 20 "$WORK/build.err" | sed 's/^/    /'
    fail=1
fi

if [ "$fail" -ne 0 ]; then
    echo "check-consumer: FAIL"
    exit 1
fi
echo "check-consumer: PASS"
