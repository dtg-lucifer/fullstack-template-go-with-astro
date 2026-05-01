#!/usr/bin/env bash
# rename-module.sh — Replace the template module path with your own.
#
# Usage:
#   ./scripts/rename-module.sh <new-module-path>
#
# Example:
#   ./scripts/rename-module.sh github.com/dtg-lucifer/everato
#
# What it does:
#   1. Reads the current module name from go.mod (no hardcoded template name)
#   2. Updates go.mod
#   3. Rewrites every import path in every .go file under the repo
#   4. Updates sqlc.yaml if it references the module path
#   5. Prints a summary of changed files
#
# Requirements: bash, sed, find, grep (standard on Linux/macOS)

set -euo pipefail

# ── Validate arguments ────────────────────────────────────────────────────────

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 <new-module-path>"
  echo "Example: $0 github.com/dtg-lucifer/everato"
  exit 1
fi

NEW_MODULE="$1"

# ── Detect current module name from go.mod ────────────────────────────────────

if [[ ! -f go.mod ]]; then
  echo "Error: go.mod not found. Run this script from the repository root."
  exit 1
fi

CURRENT_MODULE=$(grep -m1 '^module ' go.mod | awk '{print $2}')

if [[ -z "$CURRENT_MODULE" ]]; then
  echo "Error: could not read module name from go.mod."
  exit 1
fi

if [[ "$NEW_MODULE" == "$CURRENT_MODULE" ]]; then
  echo "New module path is the same as the current one — nothing to do."
  exit 0
fi

echo "Renaming module:"
echo "  FROM: $CURRENT_MODULE"
echo "  TO:   $NEW_MODULE"
echo ""

# ── Detect sed in-place flag (BSD/macOS vs GNU) ───────────────────────────────

if sed --version 2>/dev/null | grep -q GNU; then
  sedi() { sed -i "s|${CURRENT_MODULE}|${NEW_MODULE}|g" "$1"; }
else
  # macOS / BSD sed requires an explicit backup extension with -i
  sedi() { sed -i '' "s|${CURRENT_MODULE}|${NEW_MODULE}|g" "$1"; }
fi

CHANGED=0

# ── go.mod ────────────────────────────────────────────────────────────────────

if grep -q "$CURRENT_MODULE" go.mod; then
  sedi go.mod
  echo "  ✔ go.mod"
  CHANGED=$((CHANGED + 1))
fi

# ── All .go files (recursive, skip vendor/) ───────────────────────────────────

while IFS= read -r -d '' file; do
  if grep -q "$CURRENT_MODULE" "$file"; then
    sedi "$file"
    echo "  ✔ $file"
    CHANGED=$((CHANGED + 1))
  fi
done < <(find . \
  -type f \
  -name "*.go" \
  -not -path "./vendor/*" \
  -not -path "./.git/*" \
  -print0)

# ── sqlc.yaml (may contain the module path in output package settings) ────────

if [[ -f sqlc.yaml ]] && grep -q "$CURRENT_MODULE" sqlc.yaml; then
  sedi sqlc.yaml
  echo "  ✔ sqlc.yaml"
  CHANGED=$((CHANGED + 1))
fi

# ── Summary ───────────────────────────────────────────────────────────────────

echo ""
echo "Done. $CHANGED file(s) updated."
echo ""
echo "Next steps:"
echo "  go mod tidy          # sync dependencies"
echo "  make sqlc-gen        # regenerate the repository package"
echo "  make build-web       # build the Astro frontend"
echo "  make build           # verify everything compiles"
