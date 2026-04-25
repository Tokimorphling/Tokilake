#!/usr/bin/env bash

set -euo pipefail

domain=""
email=""
app_dir="/opt/tokilake"
config_source=""
image="ghcr.io/tokimorphling/tokilake:latest"
container_name="tokilake"
listen_port="19981"
timezone_value="UTC"
user_token_secret=""
sql_dsn=""
redis_conn_string=""
skip_package_install="false"
pull_image="true"
update_mode="false"
stream_include_dir="/etc/nginx/stream-conf.d"

usage() {
  cat <<'EOF'
Usage:
  sudo ./deploy/bootstrap-nginx-letsencrypt.sh --domain api.example.com --email admin@example.com [options]

First deploy:
  sudo ./deploy/bootstrap-nginx-letsencrypt.sh \
    --domain api.example.com \
    --email admin@example.com \
    --sql-dsn 'postgres://user:password@127.0.0.1:5432/tokilake'

Update image:
  sudo ./deploy/bootstrap-nginx-letsencrypt.sh --domain api.example.com --update

Options:
  --domain <domain>              Public domain name, required
  --email <email>                Let's Encrypt email, required only before the first certificate exists
  --update                       Reuse existing config/cert and rebuild the container
  --app-dir <dir>                Install root, default /opt/tokilake
  --config <path>                Config template, default ./deploy/config.production-nginx.yaml
  --image <image>                Tokilake image, default ghcr.io/tokimorphling/tokilake:latest
  --container-name <name>        Docker container name, default tokilake
  --port <port>                  Tokilake listen port, default 19981
  --tz <timezone>                Container timezone env, default UTC
  --sql-dsn <dsn>                Persist SQL DSN into config.yaml
  --redis <conn>                 Persist Redis connection string into config.yaml
  --user-token-secret <secret>   Persist USER_TOKEN_SECRET override
  --no-pull                      Do not pull the Docker image before recreating the container
  --skip-package-install         Skip apt package installation
  --help                         Show this help

This script manages the production path only:
  - host nginx terminates HTTPS and proxies HTTP/WebSocket
  - Let's Encrypt provides certificates
  - Tokilake runs in Docker
  - nginx stream forwards 443/udp to Tokilake QUIC
EOF
}

require_arg() {
  if [ "$#" -lt 2 ] || [ -z "${2:-}" ]; then
    echo "$1 requires a value" >&2
    exit 1
  fi
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --domain)
      require_arg "$@"
      domain="$2"
      shift 2
      ;;
    --email)
      require_arg "$@"
      email="$2"
      shift 2
      ;;
    --update)
      update_mode="true"
      shift
      ;;
    --app-dir)
      require_arg "$@"
      app_dir="$2"
      shift 2
      ;;
    --config)
      require_arg "$@"
      config_source="$2"
      shift 2
      ;;
    --image)
      require_arg "$@"
      image="$2"
      shift 2
      ;;
    --container-name)
      require_arg "$@"
      container_name="$2"
      shift 2
      ;;
    --port)
      require_arg "$@"
      listen_port="$2"
      shift 2
      ;;
    --tz)
      require_arg "$@"
      timezone_value="$2"
      shift 2
      ;;
    --sql-dsn)
      require_arg "$@"
      sql_dsn="$2"
      shift 2
      ;;
    --redis)
      require_arg "$@"
      redis_conn_string="$2"
      shift 2
      ;;
    --user-token-secret)
      require_arg "$@"
      user_token_secret="$2"
      shift 2
      ;;
    --no-pull)
      pull_image="false"
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

if ! [[ "$listen_port" =~ ^[0-9]+$ ]] || [ "$listen_port" -lt 1 ] || [ "$listen_port" -gt 65535 ]; then
  echo "invalid --port: $listen_port" >&2
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

cert_dir="/etc/letsencrypt/live/${domain}"
fullchain_file="${cert_dir}/fullchain.pem"
privkey_file="${cert_dir}/privkey.pem"

if [ -z "$email" ] && { [ ! -f "$fullchain_file" ] || [ ! -f "$privkey_file" ]; }; then
  echo "--email is required before the first Let's Encrypt certificate exists" >&2
  exit 1
fi

install_packages() {
  if [ "$skip_package_install" = "true" ]; then
    return
  fi

  if ! command -v apt-get >/dev/null 2>&1; then
    echo "apt-get not found; install nginx, certbot, python3, docker, and nginx stream support first" >&2
    exit 1
  fi

  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ca-certificates \
    certbot \
    curl \
    nginx \
    python3 \
    python3-certbot-nginx

  if ! command -v docker >/dev/null 2>&1; then
    DEBIAN_FRONTEND=noninteractive apt-get install -y docker.io
  fi

  DEBIAN_FRONTEND=noninteractive apt-get install -y libnginx-mod-stream || true
}

require_commands() {
  for required_cmd in docker nginx certbot python3; do
    if ! command -v "$required_cmd" >/dev/null 2>&1; then
      echo "required command not found: $required_cmd" >&2
      echo "install packages first or rerun without --skip-package-install" >&2
      exit 1
    fi
  done
}

enable_service() {
  service_name="$1"
  if command -v systemctl >/dev/null 2>&1; then
    systemctl enable --now "$service_name"
  fi
}

reload_nginx() {
  nginx -t
  if command -v systemctl >/dev/null 2>&1; then
    systemctl reload nginx || systemctl restart nginx
  else
    nginx -s reload
  fi
}

install_packages
require_commands
enable_service docker
enable_service nginx

safe_name="$(printf '%s' "$domain" | tr -c 'A-Za-z0-9_.-' '_')"
install_root="${app_dir}"
config_dir="${install_root}/config"
data_dir="${install_root}/data"
config_dest="${config_dir}/config.yaml"
webroot="/var/www/certbot"
nginx_conf=""
nginx_enabled=""
config_created="false"

if [ -d /etc/nginx/sites-available ] || [ -d /etc/nginx/sites-enabled ]; then
  mkdir -p /etc/nginx/sites-available /etc/nginx/sites-enabled
  nginx_conf="/etc/nginx/sites-available/${safe_name}.conf"
  nginx_enabled="/etc/nginx/sites-enabled/${safe_name}.conf"
elif [ -d /etc/nginx/conf.d ]; then
  nginx_conf="/etc/nginx/conf.d/${safe_name}.conf"
else
  mkdir -p /etc/nginx/conf.d
  nginx_conf="/etc/nginx/conf.d/${safe_name}.conf"
fi

mkdir -p "$config_dir" "$data_dir" "${data_dir}/logs" "$webroot"

if [ ! -f "$config_dest" ]; then
  install -m 0644 "$config_source" "$config_dest"
  config_created="true"
fi

python3 - "$config_dest" "$domain" "$listen_port" "$user_token_secret" "$sql_dsn" "$redis_conn_string" "$config_created" <<'PY'
import secrets
import sys

config_path, domain, port, token_secret_arg, sql_dsn_arg, redis_arg, config_created = sys.argv[1:8]

default_sqids_alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

placeholders = {
    "user_token_secret": "replace-with-at-least-32-random-characters",
    "session_secret": "replace-with-a-random-session-secret",
    "hashids_salt": "replace-with-a-random-unique-sqids-alphabet",
    "sql_dsn": "postgres://user:password@127.0.0.1:5432/tokilake",
}

with open(config_path, "r", encoding="utf-8") as f:
    lines = f.read().splitlines()

def normalize_value(value):
    return value.strip().strip('"').strip("'")

def is_valid_sqids_alphabet(value):
    return len(value) >= 3 and value.isascii() and len(set(value)) == len(value)

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
if hashids_salt_value in {"replace-with-a-stable-random-salt", placeholders["hashids_salt"]}:
    hashids_salt_value = generate_sqids_alphabet()
elif not hashids_salt_value and config_created == "true":
    hashids_salt_value = generate_sqids_alphabet()
elif config_created == "true" and not is_valid_sqids_alphabet(hashids_salt_value):
    hashids_salt_value = generate_sqids_alphabet()

sql_dsn_value = sql_dsn_arg or existing.get("sql_dsn", "")
if config_created == "true" and sql_dsn_value == placeholders["sql_dsn"]:
    sql_dsn_value = ""

redis_value = redis_arg or existing.get("redis_conn_string", "")

overrides = {
    "port": port,
    "server_address": f"https://{domain}",
    "user_token_secret": token_secret_value,
    "session_secret": session_secret_value,
    "hashids_salt": hashids_salt_value,
    "sql_dsn": sql_dsn_value,
    "redis_conn_string": redis_value,
}

result = []
seen = set()
for line in lines:
    if ":" not in line or line.lstrip().startswith("#"):
        result.append(line)
        continue
    raw_key, _ = line.split(":", 1)
    key = raw_key.strip()
    if key in overrides:
        value = overrides[key]
        if key == "port":
            result.append(f"port: {value}")
        else:
            escaped = str(value).replace("\\", "\\\\").replace('"', '\\"')
            result.append(f'{key}: "{escaped}"')
        seen.add(key)
        continue
    result.append(line)

for key in ("port", "server_address", "user_token_secret", "session_secret", "hashids_salt", "sql_dsn", "redis_conn_string"):
    if key in seen:
        continue
    value = overrides[key]
    if key == "port":
        result.append(f"port: {value}")
    else:
        escaped = str(value).replace("\\", "\\\\").replace('"', '\\"')
        result.append(f'{key}: "{escaped}"')

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
    listen [::]:80;
    server_name ${domain};

    location /.well-known/acme-challenge/ {
        root ${webroot};
    }

    location / {
        proxy_pass http://127.0.0.1:${listen_port};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection \$connection_upgrade;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        proxy_buffering off;
    }
}
EOF

if [ -n "$nginx_enabled" ] && [ ! -e "$nginx_enabled" ]; then
  ln -s "$nginx_conf" "$nginx_enabled"
fi

reload_nginx

if [ ! -f "$fullchain_file" ] || [ ! -f "$privkey_file" ]; then
  if [ -z "$email" ]; then
    echo "--email is required before the first Let's Encrypt certificate exists" >&2
    exit 1
  fi

  certbot certonly \
    --webroot \
    --webroot-path "$webroot" \
    --non-interactive \
    --agree-tos \
    --email "$email" \
    --keep-until-expiring \
    -d "$domain"
elif [ "$update_mode" = "true" ]; then
  certbot renew --quiet --cert-name "$domain" || true
fi

cat >"$nginx_conf" <<EOF
map \$http_upgrade \$connection_upgrade {
    default upgrade;
    '' close;
}

server {
    listen 80;
    listen [::]:80;
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
    listen [::]:443 ssl http2;
    server_name ${domain};

    ssl_certificate ${fullchain_file};
    ssl_certificate_key ${privkey_file};
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers off;

    client_max_body_size 100m;

    location / {
        proxy_pass http://127.0.0.1:${listen_port};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection \$connection_upgrade;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        proxy_buffering off;
    }
}
EOF

if [ -n "$nginx_enabled" ] && [ ! -e "$nginx_enabled" ]; then
  ln -s "$nginx_conf" "$nginx_enabled"
fi

mkdir -p "$stream_include_dir"
if ! grep -Rqs "ngx_stream_module.so" /etc/nginx/nginx.conf /etc/nginx/modules-enabled 2>/dev/null; then
  if [ -f /usr/lib/nginx/modules/ngx_stream_module.so ]; then
    if grep -q "include /etc/nginx/modules-enabled/\\*.conf;" /etc/nginx/nginx.conf; then
      mkdir -p /etc/nginx/modules-enabled
      printf '%s\n' "load_module modules/ngx_stream_module.so;" > /etc/nginx/modules-enabled/50-mod-stream.conf
    else
      python3 - /etc/nginx/nginx.conf <<'PY'
import sys

path = sys.argv[1]
with open(path, "r", encoding="utf-8") as f:
    content = f.read()
if "ngx_stream_module.so" not in content:
    content = "load_module modules/ngx_stream_module.so;\n" + content
with open(path, "w", encoding="utf-8") as f:
    f.write(content)
PY
    fi
  fi
fi

if ! grep -qF "include ${stream_include_dir}/*.conf;" /etc/nginx/nginx.conf; then
  python3 - /etc/nginx/nginx.conf "$stream_include_dir" <<'PY'
import re
import sys

path, include_dir = sys.argv[1:3]
include_line = f"include {include_dir}/*.conf;"

with open(path, "r", encoding="utf-8") as f:
    lines = f.readlines()

for i, line in enumerate(lines):
    if re.match(r"^\s*stream\s*\{", line):
        indent = re.match(r"^(\s*)", line).group(1) + "    "
        lines.insert(i + 1, f"{indent}{include_line}\n")
        break
else:
    if lines and not lines[-1].endswith("\n"):
        lines[-1] += "\n"
    lines.extend(["\n", "stream {\n", f"    {include_line}\n", "}\n"])

with open(path, "w", encoding="utf-8") as f:
    f.writelines(lines)
PY
fi

cat >"${stream_include_dir}/tokilake-${safe_name}.conf" <<EOF
server {
    listen 443 udp reuseport;
    proxy_pass 127.0.0.1:${listen_port};
    proxy_timeout 1h;
    proxy_responses 0;
}
EOF

reload_nginx

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
  -e QUIC_ENABLE=true \
  -e QUIC_PORT="$listen_port" \
  -e QUIC_CERT_FILE="/etc/letsencrypt/live/${domain}/fullchain.pem" \
  -e QUIC_KEY_FILE="/etc/letsencrypt/live/${domain}/privkey.pem" \
  -v "${data_dir}:/data" \
  -v "${config_dest}:/data/config.yaml:ro" \
  -v "/etc/letsencrypt:/etc/letsencrypt:ro" \
  "$image"

reload_nginx

cat <<EOF
Tokilake deployment complete.

Domain:      https://${domain}
Config:      ${config_dest}
Data:        ${data_dir}
Container:   ${container_name}
Image:       ${image}
HTTP/WS:     nginx 443/tcp -> 127.0.0.1:${listen_port}/tcp
QUIC:        nginx 443/udp -> 127.0.0.1:${listen_port}/udp

To update later:
  sudo ./deploy/bootstrap-nginx-letsencrypt.sh --domain ${domain} --update
EOF
