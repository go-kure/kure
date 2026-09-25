#!/usr/bin/env bash
# Generate the Go blocks of the documentation pages from compiled Example
# functions, or check that they are generated.
#
#   scripts/gen-doc-examples.sh              rewrite the generated blocks
#   scripts/gen-doc-examples.sh --check      fail on drift or an unvouched block
#   scripts/gen-doc-examples.sh --self-test  run the generator's own tests
#
# A page marks a generated block with
#
#   <!-- doc-example: <package dir> <Example function> -->
#   ...
#   <!-- doc-example:end -->
#
# and a Go block that cannot be generated -- a type declaration, a sketch with
# placeholders -- with `<!-- doc-example:excerpt <reason> -->` directly above
# its fence. On an enabled page every ```go block is one or the other, so
# every snippet a reader copies either compiles in `go test` or says why it is
# not meant to. scripts/docexamples has the marker rules and the body format.
#
# To add an example: write an Example function in the package's
# example_test.go (package <name>_test), put the two markers where the block
# goes, and run this script. To enable a page, add it to ENABLED_PAGES.
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

# The pages whose every Go block must be generated or marked as an excerpt.
ENABLED_PAGES=(
	pkg/kubernetes/README.md
	pkg/kubernetes/certmanager/README.md
	pkg/kubernetes/cilium/README.md
	pkg/kubernetes/cnpg/README.md
	pkg/kubernetes/externalsecrets/README.md
	pkg/kubernetes/fluxcd/README.md
	pkg/kubernetes/metallb/README.md
	pkg/kubernetes/prometheus/README.md
	pkg/kubernetes/volsync/README.md
)

export GOWORK="${GOWORK:-off}"
case "${1:-}" in
"") exec go run ./scripts/docexamples generate "${ENABLED_PAGES[@]}" ;;
--check) exec go run ./scripts/docexamples check "${ENABLED_PAGES[@]}" ;;
--self-test) exec go test ./scripts/docexamples ;;
*)
	printf 'usage: %s [--check|--self-test]\n' "$0" >&2
	exit 2
	;;
esac
