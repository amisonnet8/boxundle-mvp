#!/usr/bin/env bash
# Build after a Go source or module file is changed by srwr's edit / new, and
# report failures to Claude. In strict mode (srwr init) the built-in Edit and
# Write are denied, so this hook runs on the srwr tools; their input names the
# file as "file" (or as "edits[].file" for several changes at once).
set -euo pipefail

changed=false
while IFS= read -r file; do
  case "$file" in
    *.go | *go.mod | *go.sum) changed=true ;;
  esac
done < <(jq -r '.tool_input | [.file, .file_path, (.edits[]?.file)] | .[] | strings')

if [ "$changed" = false ]; then
  exit 0
fi

cd "$CLAUDE_PROJECT_DIR"
if ! output=$(qsoku build 2>&1); then
  echo "$output" >&2
  exit 2
fi
