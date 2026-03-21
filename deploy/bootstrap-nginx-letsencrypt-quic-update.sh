#!/usr/bin/env bash

set -euo pipefail

domain=""
app_dir="/opt/tokilake"
container_name="tokilake"
listen_port="19981"
timezone_value="UTC"
image=""
public_udp_port="443"
stream_include_dir="/etc/nginx/stream-conf.d"
skip_package_install="false"
pull_image="false"
timezone_explicit="false"
image_explicit="false"

usage() {
  cat <<'EOF'
Usage:
  sudo ./deploy/bootstrap-nginx-letsencrypt-quic-update.sh --domain api.example.com [options]

Options:
  --domain <domain>              Public domain name, required
  --app-dir <dir>                Install root, default /opt/tokilake
  --container-name <name>        Docker container name, default tokilake
  --port <port>                  Tokilake listen port, default 19981
  --public-udp-port <port>       Public nginx UDP port, default 443
  --image <image>                Docker image to run; defaults to current container image if present
  --tz <timezone>                Container timezone env; defaults to current container TZ if present, else UTC
  --stream-include-dir <dir>     nginx stream include dir, default /etc/nginx/stream-conf.d
  --pull                         Pull image before recreating container
  --skip-package-install         Skip apt package installation for nginx stream support
  --help                         Show this help

Behavior:
  - Adds nginx UDP stream proxy for QUIC
  - Mounts Let's Encrypt certificates into the Tokilake container
  - Recreates the container with both TCP and UDP bindings on the Tokilake port
  - Enables QUIC in Tokilake via environment variables

Assumptions:
  - Debian/Ubuntu style system with apt, systemd, nginx, docker
  - Existing HTTPS site already serves the same --domain via nginx
  - Let's Encrypt certificate already exists under /etc/letsencrypt/live/<domain>/
  - Tokilake follows the repository bootstrap layout under <app-dir>/
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --domain)
      domain="${2:-}"
      shift 2
      ;;
    --app-dir)
      app_dir="${2:-}"
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
    --public-udp-port)
      public_udp_port="${2:-}"
      shift 2
      ;;
    --image)
      image="${2:-}"
      image_explicit="true"
      shift 2
      ;;
    --tz)
      timezone_value="${2:-}"
      timezone_explicit="true"
      shift 2
      ;;
    --stream-include-dir)
      stream_include_dir="${2:-}"
      shift 2
      ;;
    --pull)
      pull_image="true"
      shift
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

if [ -z "$domain" ]; then
  usage >&2
  exit 1
fi

for required_cmd in nginx docker python3; do
  if ! command -v "$required_cmd" >/dev/null 2>&1; then
    echo "required command not found: $required_cmd" >&2
    exit 1
  fi
done

install_root="${app_dir}"
config_dest="${install_root}/config/config.yaml"
data_dir="${install_root}/data"
cert_live_dir="/etc/letsencrypt/live/${domain}"
cert_file_in_container="${cert_live_dir}/fullchain.pem"
key_file_in_container="${cert_live_dir}/privkey.pem"
stream_conf_path="${stream_include_dir}/tokilake-quic-${domain}.conf"

if [ ! -f "$config_dest" ]; then
  echo "runtime config not found: $config_dest" >&2
  exit 1
fi

if [ ! -f "$cert_file_in_container" ] || [ ! -f "$key_file_in_container" ]; then
  echo "Let's Encrypt certificate files not found under ${cert_live_dir}" >&2
  echo "expected: ${cert_file_in_container} and ${key_file_in_container}" >&2
  exit 1
fi

if docker inspect "$container_name" >/dev/null 2>&1; then
  if [ "$image_explicit" != "true" ]; then
    image="$(docker inspect -f '{{.Config.Image}}' "$container_name")"
  fi
  if [ "$timezone_explicit" != "true" ]; then
    detected_tz="$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$container_name" | awk -F= '$1=="TZ"{print $2; exit}')"
    if [ -n "$detected_tz" ]; then
      timezone_value="$detected_tz"
    fi
  fi
fi

if [ -z "$image" ]; then
  image="ghcr.io/tokimorphling/tokilake:latest"
fi

ensure_stream_module() {
  local nginx_v module_path module_conf

  nginx_v="$(nginx -V 2>&1 || true)"
  if printf '%s' "$nginx_v" | grep -q -- '--with-stream'; then
    if printf '%s' "$nginx_v" | grep -q -- '--with-stream=dynamic'; then
      if grep -Rqs 'ngx_stream_module\.so' /etc/nginx/modules-enabled /etc/nginx/nginx.conf 2>/dev/null; then
        return 0
      fi

      for candidate in \
        /usr/lib/nginx/modules/ngx_stream_module.so \
        /usr/lib64/nginx/modules/ngx_stream_module.so \
        /usr/share/nginx/modules/ngx_stream_module.so
      do
        if [ -f "$candidate" ]; then
          module_path="$candidate"
          break
        fi
      done

      if [ -z "${module_path:-}" ]; then
        if [ "$skip_package_install" = "true" ]; then
          echo "nginx stream module is not loaded and no module binary was found" >&2
          echo "install libnginx-mod-stream or rerun without --skip-package-install" >&2
          exit 1
        fi
        apt-get update
        DEBIAN_FRONTEND=noninteractive apt-get install -y libnginx-mod-stream
        for candidate in \
          /usr/lib/nginx/modules/ngx_stream_module.so \
          /usr/lib64/nginx/modules/ngx_stream_module.so \
          /usr/share/nginx/modules/ngx_stream_module.so
        do
          if [ -f "$candidate" ]; then
            module_path="$candidate"
            break
          fi
        done
      fi

      if [ -z "${module_path:-}" ]; then
        echo "unable to locate ngx_stream_module.so after installation" >&2
        exit 1
      fi

      mkdir -p /etc/nginx/modules-enabled
      module_conf="/etc/nginx/modules-enabled/50-mod-stream.conf"
      cat >"$module_conf" <<EOF
load_module ${module_path};
EOF
    fi
    return 0
  fi

  if [ "$skip_package_install" = "true" ]; then
    echo "current nginx build does not include stream support" >&2
    echo "install nginx with stream support or rerun without --skip-package-install" >&2
    exit 1
  fi

  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y nginx-full libnginx-mod-stream
}

ensure_stream_include() {
  local nginx_main_conf="/etc/nginx/nginx.conf"

  if [ ! -f "$nginx_main_conf" ]; then
    echo "nginx main config not found: $nginx_main_conf" >&2
    exit 1
  fi

  mkdir -p "$stream_include_dir"

  python3 - "$nginx_main_conf" "$stream_include_dir" <<'PY'
import pathlib
import re
import sys

nginx_conf = pathlib.Path(sys.argv[1])
stream_dir = sys.argv[2]
include_line = f"    include {stream_dir}/*.conf;"
text = nginx_conf.read_text(encoding="utf-8")

if f"include {stream_dir}/*.conf;" in text:
    sys.exit(0)

stream_match = re.search(r"(?m)^stream\s*\{", text)
if stream_match:
    insert_at = stream_match.end()
    text = text[:insert_at] + "\n" + include_line + text[insert_at:]
else:
    stream_block = f"stream {{\n{include_line}\n}}\n\n"
    http_match = re.search(r"(?m)^http\s*\{", text)
    if http_match:
        text = text[:http_match.start()] + stream_block + text[http_match.start():]
    else:
        text = text.rstrip() + "\n\n" + stream_block

nginx_conf.write_text(text, encoding="utf-8")
PY
}

write_stream_conf() {
  local upstream_name

  upstream_name="tokilake_quic_$(printf '%s' "$domain" | tr '.-' '_' | tr -cd '[:alnum:]_')"

  cat >"$stream_conf_path" <<EOF
upstream ${upstream_name} {
    server 127.0.0.1:${listen_port};
}

server {
    listen ${public_udp_port} udp reuseport;
    proxy_timeout 3600s;
    proxy_pass ${upstream_name};
}
EOF
}

recreate_container() {
  if [ "$pull_image" = "true" ]; then
    docker pull "$image"
  fi

  docker rm -f "$container_name" >/dev/null 2>&1 || true

  docker run -d \
    --name "$container_name" \
    --restart always \
    -p "127.0.0.1:${listen_port}:${listen_port}/tcp" \
    -p "127.0.0.1:${listen_port}:${listen_port}/udp" \
    -e TZ="$timezone_value" \
    -e QUIC_ENABLE="true" \
    -e QUIC_PORT="${listen_port}" \
    -e QUIC_CERT_FILE="${cert_file_in_container}" \
    -e QUIC_KEY_FILE="${key_file_in_container}" \
    -v "${config_dest}:/data/config.yaml:ro" \
    -v "${data_dir}:/data" \
    -v "/etc/letsencrypt:/etc/letsencrypt:ro" \
    "$image"
}

ensure_stream_module
ensure_stream_include
write_stream_conf
recreate_container

nginx -t
if command -v systemctl >/dev/null 2>&1; then
  systemctl reload nginx
else
  nginx -s reload
fi

echo
echo "Tokilake QUIC nginx update completed."
echo "Domain: ${domain}"
echo "Image: ${image}"
echo "Container: ${container_name}"
echo "Tokilake port: ${listen_port} (TCP+UDP on loopback)"
echo "Public QUIC port: ${public_udp_port}/udp"
echo "QUIC cert: ${cert_file_in_container}"
echo "QUIC key: ${key_file_in_container}"
echo "Stream config: ${stream_conf_path}"
echo
echo "Recommended next checks:"
echo "  docker ps --filter name=${container_name}"
echo "  docker logs ${container_name} --tail 100"
echo "  nginx -T | grep -n '${domain}'"
echo "  ss -lunp | grep ':${public_udp_port} '"
