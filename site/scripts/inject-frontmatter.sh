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

# Main documentation
inject_fm "$KURE_ROOT/README.md"                          "overview/readme.md"                          "Project README"               10
inject_fm "$KURE_ROOT/docs/ARCHITECTURE.md"                "architecture/details.md"                     "Architecture Details"          10

# Launcher package
inject_fm "$KURE_ROOT/pkg/launcher/README.md"              "packages/launcher/readme.md"                 "Overview"                     10
inject_fm "$KURE_ROOT/pkg/launcher/DESIGN.md"              "packages/launcher/design.md"                 "Design"                       20
inject_fm "$KURE_ROOT/pkg/launcher/DESIGN-DETAILS.md"      "packages/launcher/design-details.md"         "Design Details"               30
inject_fm "$KURE_ROOT/pkg/launcher/CODE-DESIGN.md"         "packages/launcher/code-design.md"            "Code Design"                  40
inject_fm "$KURE_ROOT/pkg/launcher/CODE-IMPLEMENTATION-PLAN.md" "packages/launcher/code-implementation-plan.md" "Implementation Plan"    50
inject_fm "$KURE_ROOT/pkg/launcher/ARCHITECTURE.md"        "packages/launcher/architecture.md"           "Architecture"                 60

# Patch package
inject_fm "$KURE_ROOT/pkg/patch/DESIGN.md"                 "packages/patch/design.md"                    "Design"                       10
inject_fm "$KURE_ROOT/pkg/patch/ERROR_HANDLING.md"          "packages/patch/error-handling.md"            "Error Handling"               20
inject_fm "$KURE_ROOT/pkg/patch/PATCH_ENGINE_DESIGN.md"     "packages/patch/patch-engine-design.md"       "Patch Engine Design"          30
inject_fm "$KURE_ROOT/pkg/patch/PATH_RESOLUTION.md"         "packages/patch/path-resolution.md"           "Path Resolution"              40

# Stack package
inject_fm "$KURE_ROOT/pkg/stack/DESIGN.md"                 "packages/stack/design.md"                    "Design"                       10
inject_fm "$KURE_ROOT/pkg/stack/STATUS.md"                  "packages/stack/status.md"                    "Status"                       20

# Stack generators
inject_fm "$KURE_ROOT/pkg/stack/generators/DESIGN.md"      "packages/stack/generators/design.md"         "Design"                       10
inject_fm "$KURE_ROOT/pkg/stack/generators/ARCHITECTURE.md" "packages/stack/generators/architecture.md"  "Architecture"                 20

# Layout package
inject_fm "$KURE_ROOT/pkg/stack/layout/README.md"          "packages/layout/readme.md"                   "Overview"                     10

# Getting started
inject_fm "$KURE_ROOT/docs/quickstart.md"                  "getting-started/quickstart.md"               "Quickstart"                   10

# Architecture
inject_fm "$KURE_ROOT/docs/plugin-architecture-design.md"  "architecture/plugin-design.md"               "Plugin Architecture"          20
inject_fm "$KURE_ROOT/docs/ux-design.md"                   "architecture/ux-design.md"                   "UX Design"                    30

# Reference
inject_fm "$KURE_ROOT/docs/compatibility.md"               "reference/compatibility.md"                  "Compatibility Matrix"         10

# Examples
inject_fm "$KURE_ROOT/examples/patches/README.md"          "examples/patches.md"                         "Patches"                      10
inject_fm "$KURE_ROOT/examples/generators/README.md"        "examples/generators.md"                     "Generators"                   20
inject_fm "$KURE_ROOT/examples/kurel/frigate/README.md"     "examples/kurel-frigate.md"                  "Kurel Frigate"                30
inject_fm "$KURE_ROOT/examples/validation/README.md"        "examples/validation.md"                     "Validation"                   40

# Development
inject_fm "$KURE_ROOT/DEVELOPMENT.md"                      "development/guide.md"                        "Development Guide"            10
inject_fm "$KURE_ROOT/docs/github-workflows.md"            "development/github-workflows.md"             "GitHub Workflows"             20

# Changelog
inject_fm "$KURE_ROOT/CHANGELOG.md"                        "changelog/releases.md"                       "Releases"                     10

echo "Front matter injection complete. Output in $GEN_DIR"
