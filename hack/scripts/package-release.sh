#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 6 ]; then
  echo "usage: $0 <app> <version> <os> <arch> <binary-path> <output-dir>" >&2
  exit 1
fi

app="$1"
version="$2"
target_os="$3"
target_arch="$4"
binary_path="$5"
output_dir="$6"

if [ ! -f "$binary_path" ]; then
  echo "binary not found: $binary_path" >&2
  exit 1
fi

archive_ext="tar.gz"
if [ "$target_os" = "windows" ]; then
  archive_ext="zip"
fi

archive_name="${app}_${version}_${target_os}_${target_arch}.${archive_ext}"
archive_path="${output_dir}/${archive_name}"

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

payload_dir="${work_dir}/${app}_${version}_${target_os}_${target_arch}"
mkdir -p "$payload_dir"

cp "$binary_path" "${payload_dir}/$(basename "$binary_path")"
cp LICENSE "${payload_dir}/LICENSE"

case "$app" in
  tokilake)
    cp config.example.yaml "${payload_dir}/config.example.yaml"
    ;;
  tokiame)
    cp packaging/tokiame.json.example "${payload_dir}/tokiame.json.example"
    ;;
  *)
    echo "unsupported app: $app" >&2
    exit 1
    ;;
esac

mkdir -p "$output_dir"

if [ "$target_os" = "windows" ]; then
  powershell -NoProfile -NonInteractive -Command "Compress-Archive -Path '${payload_dir}/*' -DestinationPath '${archive_path}' -Force" >/dev/null
else
  tar -C "$payload_dir" -czf "$archive_path" .
fi

echo "$archive_path"
