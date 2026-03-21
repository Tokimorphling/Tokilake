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
  sudo ./deploy/bootstrap-nginx-letsencrypt.sh --domain api.example.com --email admin@example.com [options]

Options:
  --domain <domain>              Public domain name, required
  --email <email>                Let's Encrypt registration email, required
  --app-dir <dir>                Install root, default /opt/tokilake
  --config <path>                Config template path, default ./deploy/config.production-nginx.yaml
  --image <image>                Tokilake image, default ghcr.io/tokimorphling/tokilake:latest
  --container-name <name>        Docker container name, default tokilake
  --port <port>                  Tokilake listen port, default 19981
  --tz <timezone>                Container timezone env, default UTC
  --user-token-secret <secret>   Persisted USER_TOKEN_SECRET override
  --skip-package-install         Skip apt package installation
  --help                         Show this help

Assumptions:
  - Debian/Ubuntu style system with apt, systemd, nginx, certbot, docker
  - Tokilake runs as a Docker container behind nginx on the same host
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

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [ -z "$config_source" ]; then
  config_source="${repo_root}/deploy/config.production-nginx.yaml"
fi

if [ ! -f "$config_source" ]; then
  echo "config template not found: $config_source" >&2
  exit 1
fi

if [ "$skip_package_install" != "true" ]; then
  packages=(nginx certbot python3 python3-certbot-nginx)
  if ! command -v docker >/dev/null 2>&1; then
    packages=(docker.io "${packages[@]}")
  fi
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y "${packages[@]}"
fi

for required_cmd in docker nginx certbot systemctl python3; do
  if ! command -v "$required_cmd" >/dev/null 2>&1; then
    echo "required command not found: $required_cmd" >&2
    echo "install packages first or rerun without --skip-package-install" >&2
    exit 1
  fi
done

systemctl enable --now docker

install_root="${app_dir}"
config_dir="${install_root}/config"
data_dir="${install_root}/data"
config_dest="${config_dir}/config.yaml"
webroot="/var/www/certbot"
nginx_conf=""
nginx_enabled=""
nginx_layout=""
config_created="false"

if [ -d /etc/nginx/sites-available ] && [ -d /etc/nginx/sites-enabled ]; then
  nginx_conf="/etc/nginx/sites-available/${domain}.conf"
  nginx_enabled="/etc/nginx/sites-enabled/${domain}.conf"
  nginx_layout="debian"
elif [ -d /etc/nginx/conf.d ]; then
  nginx_conf="/etc/nginx/conf.d/${domain}.conf"
  nginx_layout="conf.d"
else
  echo "unsupported nginx config layout under /etc/nginx" >&2
  echo "expected either sites-available/sites-enabled or conf.d" >&2
  exit 1
fi

mkdir -p "$config_dir" "$data_dir" "${data_dir}/logs" "$webroot" \
         "/etc/nginx/sites-available" "/etc/nginx/sites-enabled"

if [ ! -f "$config_dest" ]; then
  install -m 0644 "$config_source" "$config_dest"
  config_created="true"
fi

python3 - "$config_dest" "$domain" "$listen_port" "$user_token_secret" "$config_created" <<'PY'
import secrets
import sys

config_path, domain, port, token_secret_arg, config_created = sys.argv[1:6]

default_sqids_alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

with open(config_path, "r", encoding="utf-8") as f:
    lines = f.read().splitlines()

placeholders = {
    "user_token_secret": "replace-with-at-least-32-random-characters",
    "session_secret": "replace-with-a-random-session-secret",
    "hashids_salt": "replace-with-a-random-unique-sqids-alphabet",
}

overrides = {
    "port": port,
    "server_address": f"https://{domain}",
}

def normalize_value(value):
    return value.strip().strip('"').strip("'")

def is_valid_sqids_alphabet(value):
    return (
        len(value) >= 3
        and value.isascii()
        and len(set(value)) == len(value)
    )

def generate_sqids_alphabet():
    rng = secrets.SystemRandom()
    return "".join(rng.sample(list(default_sqids_alphabet), len(default_sqids_alphabet)))

existing = {}
for line in lines:
    if ":" not in line or line.lstrip().startswith("#"):
        continue
    key, value = line.split(":", 1)
    existing[key.strip()] = normalize_value(value)

token_secret_value = token_secret_arg or existing.get("user_token_secret", "")
if not token_secret_value or token_secret_value == placeholders["user_token_secret"]:
    token_secret_value = secrets.token_hex(32)

session_secret_value = existing.get("session_secret", "")
if not session_secret_value or session_secret_value == placeholders["session_secret"]:
    session_secret_value = secrets.token_hex(32)

hashids_salt_value = existing.get("hashids_salt", "")
if hashids_salt_value == "replace-with-a-stable-random-salt" or hashids_salt_value == placeholders["hashids_salt"]:
    hashids_salt_value = generate_sqids_alphabet()
elif not hashids_salt_value:
    if config_created == "true":
        hashids_salt_value = generate_sqids_alphabet()
elif config_created == "true" and not is_valid_sqids_alphabet(hashids_salt_value):
    hashids_salt_value = generate_sqids_alphabet()

overrides["user_token_secret"] = token_secret_value
overrides["session_secret"] = session_secret_value
overrides["hashids_salt"] = hashids_salt_value

result = []
seen = set()
for line in lines:
    if ":" not in line or line.lstrip().startswith("#"):
      result.append(line)
      continue
    key, value = line.split(":", 1)
    key = key.strip()
    if key in overrides:
      result.append(f'{key}: "{overrides[key]}"' if key != "port" else f"port: {overrides[key]}")
      seen.add(key)
      continue
    result.append(line)

for key in ("port", "server_address", "user_token_secret", "session_secret", "hashids_salt"):
    if key in seen:
        continue
    value = overrides[key]
    result.append(f'{key}: "{value}"' if key != "port" else f"port: {value}")

with open(config_path, "w", encoding="utf-8") as f:
    f.write("\n".join(result) + "\n")
PY

cat >"$nginx_conf" <<EOF
map \$http_upgrade \$connection_upgrade {
    default upgrade;
    '' close;
}

server {
    listen 80;
    server_name ${domain};

    client_max_body_size 200m;

    location /.well-known/acme-challenge/ {
        root ${webroot};
    }

    location / {
        proxy_pass http://127.0.0.1:${listen_port};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto http;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection \$connection_upgrade;
        proxy_buffering off;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
EOF

if [ "$nginx_layout" = "debian" ]; then
  ln -sfn "$nginx_conf" "$nginx_enabled"
  rm -f /etc/nginx/sites-enabled/default
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

nginx -t
systemctl enable --now nginx
systemctl reload nginx

certbot certonly \
  --webroot \
  --webroot-path "$webroot" \
  --domain "$domain" \
  --email "$email" \
  --agree-tos \
  --no-eff-email \
  --non-interactive

cat >"$nginx_conf" <<EOF
map \$http_upgrade \$connection_upgrade {
    default upgrade;
    '' close;
}

server {
    listen 80;
    server_name ${domain};

    location /.well-known/acme-challenge/ {
        root ${webroot};
    }

    location / {
        return 301 https://\$host\$request_uri;
    }
}

server {
    listen 443 ssl http2;
    server_name ${domain};

    ssl_certificate /etc/letsencrypt/live/${domain}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/${domain}/privkey.pem;

    client_max_body_size 200m;

    location / {
        proxy_pass http://127.0.0.1:${listen_port};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection \$connection_upgrade;
        proxy_buffering off;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
EOF

quic_update_script="${repo_root}/deploy/bootstrap-nginx-letsencrypt-quic-update.sh"
if [ ! -f "$quic_update_script" ]; then
  echo "quic update script not found: $quic_update_script" >&2
  exit 1
fi

quic_args=(
  --domain "$domain"
  --app-dir "$app_dir"
  --container-name "$container_name"
  --port "$listen_port"
  --image "$image"
  --tz "$timezone_value"
)

if [ "$skip_package_install" = "true" ]; then
  quic_args+=(--skip-package-install)
fi

bash "$quic_update_script" "${quic_args[@]}"

echo
echo "Tokilake deployment completed."
echo "Domain: https://${domain}"
echo "Image: ${image}"
echo "Container: ${container_name}"
echo "Config: ${config_dest}"
echo "Data dir: ${data_dir}"
echo "QUIC: enabled on 443/udp via nginx stream proxy"
echo
echo "Generated secrets were persisted into the config file if placeholders were present."
echo "Existing secrets were preserved."
echo
echo "Recommended next checks:"
echo "  docker ps --filter name=${container_name}"
echo "  docker logs ${container_name} --tail 100"
echo "  systemctl status nginx"
