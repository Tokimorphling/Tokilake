#!/usr/bin/env bash

set -euo pipefail

domain=""
email=""
app_dir="/opt/tokilake"
config_source=""
image="ghcr.io/tokimorphling/tokilake:latest"
container_name="tokilake"
listen_port="19981"
skip_package_install="false"
timezone_value="UTC"
user_token_secret=""

usage() {
  cat <<'EOF'
Usage:
  sudo ./deploy/bootstrap-nginx-letsencrypt-update.sh --domain api.example.com --email admin@example.com [options]

Options:
  --domain <domain>              Public domain name, required for CLI compatibility
  --email <email>                Let's Encrypt registration email, required for CLI compatibility
  --app-dir <dir>                Install root, default /opt/tokilake
  --config <path>                Accepted for CLI compatibility, not used during update
  --image <image>                Tokilake image, default ghcr.io/tokimorphling/tokilake:latest
  --container-name <name>        Docker container name, default tokilake
  --port <port>                  Tokilake listen port, default 19981
  --tz <timezone>                Container timezone env, default UTC
  --user-token-secret <secret>   Accepted for CLI compatibility, not used during update
  --skip-package-install         Accepted for CLI compatibility, not used during update
  --help                         Show this help

Behavior:
  - Only pulls the Docker image and recreates the Tokilake container
  - Does not install packages, modify nginx, request certificates, or rewrite config files
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

config_dest="${app_dir}/config/config.yaml"
data_dir="${app_dir}/data"

if ! command -v docker >/dev/null 2>&1; then
  echo "required command not found: docker" >&2
  exit 1
fi

if [ ! -f "$config_dest" ]; then
  echo "runtime config not found: $config_dest" >&2
  exit 1
fi

docker pull "$image"
docker rm -f "$container_name" >/dev/null 2>&1 || true

docker run -d \
  --name "$container_name" \
  --restart always \
  -p "127.0.0.1:${listen_port}:${listen_port}" \
  -e TZ="$timezone_value" \
  -v "${config_dest}:/data/config.yaml:ro" \
  -v "${data_dir}:/data" \
  "$image"

echo
echo "Tokilake image update completed."
echo "Image: ${image}"
echo "Container: ${container_name}"
echo "Config: ${config_dest}"
echo "Data dir: ${data_dir}"
echo
echo "Recommended next checks:"
echo "  docker ps --filter name=${container_name}"
echo "  docker logs ${container_name} --tail 100"
