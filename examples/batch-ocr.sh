#!/usr/bin/env bash
#
# Batch OCR: run askit against every *.png in a directory, writing
# side-by-side *.md files. Exits non-zero if any invocation fails.
#
# Usage: batch-ocr.sh <directory> [preset]
set -euo pipefail

dir="${1:?usage: batch-ocr.sh <directory> [preset]}"
preset="${2:-ocr-md}"

fail=0
for img in "$dir"/*.png; do
  [ -e "$img" ] || { echo "no *.png in $dir" >&2; exit 2; }
  out="${img%.png}.md"
  if ! askit query -p "$preset" --force -o "$out" "@${img}"; then
    code=$?
    case $code in
      3) echo "config error on $img" >&2 ;;
      4) echo "file error on $img" >&2 ;;
      5) echo "network error on $img (endpoint down?)" >&2 ;;
      6) echo "API error on $img" >&2 ;;
      7) echo "timeout on $img" >&2 ;;
      *) echo "unknown failure ($code) on $img" >&2 ;;
    esac
    fail=1
    continue
  fi
  echo "ok: $out"
done

exit "$fail"
