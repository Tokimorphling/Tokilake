#!/usr/bin/env bash

set -euo pipefail

domain=""
email=""
app_dir="/opt/tokilake"
config_source=""
image=""
container_name="tokilake"
listen_port="19981"
skip_package_install="false"
timezone_value="UTC"
user_token_secret=""
timezone_explicit="false"

usage() {
  cat <<'EOF'
Usage:
  sudo ./deploy/bootstrap-nginx-letsencrypt-update.sh --domain api.example.com --email admin@example.com [options]

Options:
  --domain <domain>              Public domain name, required for CLI compatibility
  --email <email>                Let's Encrypt registration email, required for CLI compatibility
  --app-dir <dir>                Install root, default /opt/tokilake
  --config <path>                Accepted for CLI compatibility, not used during update
  --image <image>                Docker image override; defaults to current container image if omitted
  --container-name <name>        Docker container name, default tokilake
  --port <port>                  Tokilake listen port, default 19981
  --tz <timezone>                Container timezone env; defaults to current container TZ if omitted
  --user-token-secret <secret>   Accepted for CLI compatibility, not used during update
  --skip-package-install         Skip apt package installation for nginx stream support
  --help                         Show this help

Behavior:
  - Recreates the Tokilake container
  - Preserves or updates QUIC nginx stream proxy configuration
  - Re-enables container QUIC bindings and certificate mounts
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --domain)
      domain="${2:-}"
      shift 2
      ;;
    --email)
      email="${2:-}"
      shift 2
      ;;
    --app-dir)
      app_dir="${2:-}"
      shift 2
      ;;
    --config)
      config_source="${2:-}"
      shift 2
      ;;
    --image)
      image="${2:-}"
      shift 2
      ;;
    --container-name)
      container_name="${2:-}"
      shift 2
      ;;
    --port)
      listen_port="${2:-}"
      shift 2
      ;;
    --tz)
      timezone_value="${2:-}"
      timezone_explicit="true"
      shift 2
      ;;
    --user-token-secret)
      user_token_secret="${2:-}"
      shift 2
      ;;
    --skip-package-install)
      skip_package_install="true"
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [ "$(id -u)" -ne 0 ]; then
  echo "this script must run as root" >&2
  exit 1
fi

if [ -z "$domain" ] || [ -z "$email" ]; then
  usage >&2
  exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
quic_update_script="${repo_root}/deploy/bootstrap-nginx-letsencrypt-quic-update.sh"

if [ ! -f "$quic_update_script" ]; then
  echo "quic update script not found: $quic_update_script" >&2
  exit 1
fi

args=(
  --domain "$domain"
  --app-dir "$app_dir"
  --container-name "$container_name"
  --port "$listen_port"
  --pull
)

if [ -n "$image" ]; then
  args+=(--image "$image")
fi

if [ "$timezone_explicit" = "true" ]; then
  args+=(--tz "$timezone_value")
fi

if [ "$skip_package_install" = "true" ]; then
  args+=(--skip-package-install)
fi

exec bash "$quic_update_script" "${args[@]}"
