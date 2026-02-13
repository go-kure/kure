#!/usr/bin/env bash
# inject-frontmatter.sh — Prepend Hugo front matter to kure markdown files.
#
# Kure docs lack Hugo front matter. This script reads a declarative mapping
# and writes processed copies into .generated/ for Hugo to mount as content.
#
# Usage: bash scripts/inject-frontmatter.sh [KURE_ROOT]
#   KURE_ROOT defaults to .. (parent of site/)

set -euo pipefail

SITE_DIR="$(cd "$(dirname "$0")/.." && pwd)"
KURE_ROOT="${1:-$(cd "$SITE_DIR/.." && pwd)}"
GEN_DIR="$SITE_DIR/.generated"

rm -rf "$GEN_DIR"
mkdir -p "$GEN_DIR"

# inject_fm SOURCE_FILE TARGET_PATH TITLE WEIGHT
#   Copies SOURCE_FILE into .generated/TARGET_PATH with Hugo front matter prepended.
inject_fm() {
  local src="$1" target="$2" title="$3" weight="$4"
  local dest="$GEN_DIR/$target"

  if [[ ! -f "$src" ]]; then
    echo "WARNING: source file not found: $src" >&2
    return 0
  fi

  mkdir -p "$(dirname "$dest")"
  {
    echo "---"
    echo "title: \"$title\""
    echo "weight: $weight"
    echo "---"
    echo ""
    cat "$src"
  } > "$dest"
}

# ─── Mapping: source_path → target_path | title | weight ───

# Getting started
inject_fm "$KURE_ROOT/docs/quickstart.md"                  "getting-started/quickstart.md"               "Quickstart"                   10

# Concepts
inject_fm "$KURE_ROOT/docs/ARCHITECTURE.md"                 "concepts/architecture.md"                    "Architecture"                 10

# Examples
inject_fm "$KURE_ROOT/examples/patches/README.md"           "examples/patches.md"                         "Patches"                      10
inject_fm "$KURE_ROOT/examples/generators/README.md"        "examples/generators.md"                      "Generators"                   20
inject_fm "$KURE_ROOT/examples/kurel/frigate/README.md"     "examples/kurel-frigate.md"                   "Kurel Frigate"                30
inject_fm "$KURE_ROOT/examples/validation/README.md"        "examples/validation.md"                      "Validation"                   40

# API Reference — package READMEs
inject_fm "$KURE_ROOT/pkg/stack/README.md"                  "api-reference/stack.md"                      "Stack"                        10
inject_fm "$KURE_ROOT/pkg/stack/fluxcd/README.md"           "api-reference/flux-engine.md"                "Flux Engine"                  20
inject_fm "$KURE_ROOT/pkg/stack/generators/README.md"       "api-reference/generators.md"                 "Generators"                   30
inject_fm "$KURE_ROOT/pkg/stack/layout/README.md"           "api-reference/layout.md"                     "Layout Engine"                40
inject_fm "$KURE_ROOT/pkg/launcher/README.md"               "api-reference/launcher.md"                   "Launcher"                     50
inject_fm "$KURE_ROOT/pkg/patch/README.md"                  "api-reference/patch.md"                      "Patch"                        60
inject_fm "$KURE_ROOT/pkg/io/README.md"                     "api-reference/io.md"                         "IO"                           70
inject_fm "$KURE_ROOT/pkg/errors/README.md"                 "api-reference/errors.md"                     "Errors"                       80
inject_fm "$KURE_ROOT/pkg/cli/README.md"                    "api-reference/cli.md"                        "CLI Utilities"                90
inject_fm "$KURE_ROOT/pkg/kubernetes/README.md"             "api-reference/kubernetes-builders.md"        "Kubernetes Builders"           95
inject_fm "$KURE_ROOT/pkg/kubernetes/fluxcd/README.md"      "api-reference/fluxcd-builders.md"            "FluxCD Builders"              100
inject_fm "$KURE_ROOT/pkg/logger/README.md"                 "api-reference/logger.md"                     "Logger"                       110
inject_fm "$KURE_ROOT/docs/compatibility.md"                "api-reference/compatibility.md"              "Compatibility Matrix"         120

# Contributing
inject_fm "$KURE_ROOT/DEVELOPMENT.md"                      "contributing/guide.md"                       "Development Guide"            10
inject_fm "$KURE_ROOT/docs/github-workflows.md"            "contributing/github-workflows.md"            "GitHub Workflows"             20

# Changelog
inject_fm "$KURE_ROOT/CHANGELOG.md"                        "changelog/releases.md"                       "Releases"                     10

echo "Front matter injection complete. Output in $GEN_DIR"
