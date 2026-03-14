#!/usr/bin/env bash

set -euo pipefail

repo="Tokimorphling/Tokilake"
version=""
darwin_amd64=""
darwin_arm64=""
linux_amd64=""
linux_arm64=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --repo)
      repo="$2"
      shift 2
      ;;
    --version)
      version="$2"
      shift 2
      ;;
    --darwin-amd64)
      darwin_amd64="$2"
      shift 2
      ;;
    --darwin-arm64)
      darwin_arm64="$2"
      shift 2
      ;;
    --linux-amd64)
      linux_amd64="$2"
      shift 2
      ;;
    --linux-arm64)
      linux_arm64="$2"
      shift 2
      ;;
    *)
      echo "unknown flag: $1" >&2
      exit 1
      ;;
  esac
done

if [ -z "$version" ] || [ -z "$darwin_amd64" ] || [ -z "$darwin_arm64" ] || [ -z "$linux_amd64" ] || [ -z "$linux_arm64" ]; then
  echo "missing required arguments" >&2
  exit 1
fi

cat <<EOF
class Tokiame < Formula
  desc "Tokilake worker for self-hosted model backends"
  homepage "https://github.com/${repo}"
  license "Apache-2.0"
  version "${version}"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/${repo}/releases/download/v${version}/tokiame_${version}_darwin_arm64.tar.gz"
      sha256 "${darwin_arm64}"
    else
      url "https://github.com/${repo}/releases/download/v${version}/tokiame_${version}_darwin_amd64.tar.gz"
      sha256 "${darwin_amd64}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/${repo}/releases/download/v${version}/tokiame_${version}_linux_arm64.tar.gz"
      sha256 "${linux_arm64}"
    else
      url "https://github.com/${repo}/releases/download/v${version}/tokiame_${version}_linux_amd64.tar.gz"
      sha256 "${linux_amd64}"
    end
  end

  def install
    bin.install "tokiame"
    pkgshare.install "tokiame.json.example"
  end

  def caveats
    <<~EOS
      Tokiame looks for its config at ~/.tokilake/tokiame.json by default.

      Example:
        mkdir -p ~/.tokilake
        cp #{pkgshare}/tokiame.json.example ~/.tokilake/tokiame.json
        tokiame
    EOS
  end

  test do
    assert_predicate bin/"tokiame", :exist?
  end
end
EOF
