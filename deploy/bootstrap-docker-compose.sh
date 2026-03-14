#!/usr/bin/env bash

set -euo pipefail

domain="localhost"
scheme="http"
http_port="80"
timezone_value="UTC"
image="ghcr.io/tokimorphling/tokilake:latest"
compose_root=""
config_source=""
env_dest=""
postgres_db="tokilake"
postgres_user="tokilake"
postgres_password=""
user_token_secret=""
session_secret=""
hashids_salt=""

usage() {
  cat <<'EOF'
Usage:
  ./deploy/bootstrap-docker-compose.sh [options]

Options:
  --domain <domain>              Public domain or hostname, default localhost
  --scheme <http|https>          External scheme written into server_address, default http
  --http-port <port>             Published nginx HTTP port, default 80
  --tz <timezone>                Container timezone env, default UTC
  --image <image>                Tokilake image, default ghcr.io/tokimorphling/tokilake:latest
  --compose-root <dir>           Runtime root, default <repo>/deploy/runtime
  --config <path>                Config template path, default ./deploy/config.compose-nginx.yaml
  --env-file <path>              Output env file, default ./deploy/.env.compose
  --postgres-db <name>           Postgres database name, default tokilake
  --postgres-user <name>         Postgres user, default tokilake
  --postgres-password <value>    Persisted Postgres password override
  --user-token-secret <secret>   Persisted USER_TOKEN_SECRET override
  --session-secret <secret>      Persisted SESSION_SECRET override
  --hashids-salt <salt>          Persisted hashids_salt override
  --help                         Show this help
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --domain)
      domain="${2:-}"
      shift 2
      ;;
    --scheme)
      scheme="${2:-}"
      shift 2
      ;;
    --http-port)
      http_port="${2:-}"
      shift 2
      ;;
    --tz)
      timezone_value="${2:-}"
      shift 2
      ;;
    --image)
      image="${2:-}"
      shift 2
      ;;
    --compose-root)
      compose_root="${2:-}"
      shift 2
      ;;
    --config)
      config_source="${2:-}"
      shift 2
      ;;
    --env-file)
      env_dest="${2:-}"
      shift 2
      ;;
    --postgres-db)
      postgres_db="${2:-}"
      shift 2
      ;;
    --postgres-user)
      postgres_user="${2:-}"
      shift 2
      ;;
    --postgres-password)
      postgres_password="${2:-}"
      shift 2
      ;;
    --user-token-secret)
      user_token_secret="${2:-}"
      shift 2
      ;;
    --session-secret)
      session_secret="${2:-}"
      shift 2
      ;;
    --hashids-salt)
      hashids_salt="${2:-}"
      shift 2
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

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [ -z "$compose_root" ]; then
  compose_root="${repo_root}/deploy/runtime"
fi
if [ -z "$config_source" ]; then
  config_source="${repo_root}/deploy/config.compose-nginx.yaml"
fi
if [ -z "$env_dest" ]; then
  env_dest="${repo_root}/deploy/.env.compose"
fi

if [ ! -f "$config_source" ]; then
  echo "config template not found: $config_source" >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "docker compose plugin is required" >&2
  exit 1
fi

mkdir -p \
  "${compose_root}/config" \
  "${compose_root}/data" \
  "${compose_root}/data/logs" \
  "${compose_root}/postgres" \
  "${compose_root}/redis"

config_dest="${compose_root}/config/config.yaml"
if [ ! -f "$config_dest" ]; then
  install -m 0644 "$config_source" "$config_dest"
fi

postgres_password_value="$(
python3 - "$config_dest" "$domain" "$scheme" "$postgres_db" "$postgres_user" "$postgres_password" "$user_token_secret" "$session_secret" "$hashids_salt" <<'PY'
import secrets
import sys

(
    config_path,
    domain,
    scheme,
    postgres_db,
    postgres_user,
    postgres_password_arg,
    token_secret_arg,
    session_secret_arg,
    hashids_salt_arg,
) = sys.argv[1:10]

with open(config_path, "r", encoding="utf-8") as f:
    lines = f.read().splitlines()

placeholders = {
    "user_token_secret": "replace-with-at-least-32-random-characters",
    "session_secret": "replace-with-a-random-session-secret",
    "hashids_salt": "replace-with-a-stable-random-salt",
    "sql_dsn": "postgres://tokilake:replace-with-db-password@postgres:5432/tokilake?sslmode=disable",
}

def normalize_value(value):
    return value.strip().strip('"').strip("'")

existing = {}
for line in lines:
    if ":" not in line or line.lstrip().startswith("#"):
        continue
    key, value = line.split(":", 1)
    existing[key.strip()] = normalize_value(value)

postgres_password = postgres_password_arg or ""
if not postgres_password:
    current_dsn = existing.get("sql_dsn", "")
    marker = f"postgres://{postgres_user}:"
    if current_dsn.startswith(marker) and "@postgres:5432/" in current_dsn:
        postgres_password = current_dsn[len(marker):].split("@postgres:5432/", 1)[0]
if not postgres_password or postgres_password == "replace-with-db-password":
    postgres_password = secrets.token_hex(24)

token_secret_value = token_secret_arg or existing.get("user_token_secret", "")
if not token_secret_value or token_secret_value == placeholders["user_token_secret"]:
    token_secret_value = secrets.token_hex(32)

session_secret_value = session_secret_arg or existing.get("session_secret", "")
if not session_secret_value or session_secret_value == placeholders["session_secret"]:
    session_secret_value = secrets.token_hex(32)

hashids_salt_value = hashids_salt_arg or existing.get("hashids_salt", "")
if not hashids_salt_value or hashids_salt_value == placeholders["hashids_salt"]:
    hashids_salt_value = secrets.token_hex(24)

overrides = {
    "server_address": f"{scheme}://{domain}",
    "user_token_secret": token_secret_value,
    "session_secret": session_secret_value,
    "hashids_salt": hashids_salt_value,
    "sql_dsn": f"postgres://{postgres_user}:{postgres_password}@postgres:5432/{postgres_db}?sslmode=disable",
}

result = []
seen = set()
for line in lines:
    if ":" not in line or line.lstrip().startswith("#"):
        result.append(line)
        continue
    key, value = line.split(":", 1)
    key = key.strip()
    if key in overrides:
      result.append(f'{key}: "{overrides[key]}"')
      seen.add(key)
      continue
    result.append(line)

for key in ("server_address", "user_token_secret", "session_secret", "hashids_salt", "sql_dsn"):
    if key not in seen:
        result.append(f'{key}: "{overrides[key]}"')

with open(config_path, "w", encoding="utf-8") as f:
    f.write("\n".join(result) + "\n")

print(postgres_password)
PY
)"

cat >"$env_dest" <<EOF
TOKILAKE_IMAGE=${image}
TOKILAKE_SERVER_NAME=${domain}
TOKILAKE_UPSTREAM=http://tokilake:19981
TOKILAKE_COMPOSE_ROOT=${compose_root}
HTTP_PORT=${http_port}
TZ=${timezone_value}
POSTGRES_DB=${postgres_db}
POSTGRES_USER=${postgres_user}
POSTGRES_PASSWORD=${postgres_password_value}
EOF

docker compose \
  --env-file "$env_dest" \
  -f "${repo_root}/deploy/docker-compose.nginx.yaml" \
  up -d

echo "Compose stack is up."
echo "Config: ${config_dest}"
echo "Env: ${env_dest}"
echo "Visit: ${scheme}://${domain}:${http_port}"
