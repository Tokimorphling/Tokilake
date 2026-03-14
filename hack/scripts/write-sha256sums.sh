#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "usage: $0 <output-file> <artifact> [artifact...]" >&2
  exit 1
fi

output_file="$1"
shift

sum_cmd=""
if command -v sha256sum >/dev/null 2>&1; then
  sum_cmd="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  sum_cmd="shasum -a 256"
else
  echo "no sha256 tool found" >&2
  exit 1
fi

tmp_output="$(mktemp)"
trap 'rm -f "$tmp_output"' EXIT

: >"$tmp_output"

for artifact_path in "$@"; do
  artifact_dir="$(dirname "$artifact_path")"
  artifact_name="$(basename "$artifact_path")"
  (
    cd "$artifact_dir"
    eval "$sum_cmd \"\$artifact_name\""
  ) >>"$tmp_output"
done

mv "$tmp_output" "$output_file"
