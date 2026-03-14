# Deploy Templates

This directory contains production deployment templates for Tokilake.

- `tokilake.service`: `systemd` unit template for running the backend on port `19981`
- `nginx.tokilake.conf`: `nginx` reverse proxy template with TLS termination, WebSocket upgrade, and streaming-friendly defaults
- `nginx.stream.tokilake.conf`: `nginx stream` TCP proxy example with PROXY protocol
- `config.local-test.yaml`: local single-machine template with direct access and proxy trust disabled
- `config.production-nginx.yaml`: production template for Tokilake behind nginx on port `19981`
- `bootstrap-nginx-letsencrypt.sh`: one-shot bootstrap script for Debian/Ubuntu, Docker, nginx, and Let's Encrypt
- `docker-compose.nginx.yaml`: compose stack for `nginx + tokilake + postgres + redis`
- `docker-compose.nginx-letsencrypt.yaml`: compose stack for `nginx + tokilake + postgres + redis + certbot`
- `config.compose-nginx.yaml`: Tokilake config template for the compose stack
- `nginx.compose.http.conf.template`: nginx template rendered inside the compose nginx container
- `nginx.compose.acme.conf.template`: temporary nginx config for HTTP-01 certificate issuance
- `nginx.compose.https.conf.template`: nginx config for HTTPS termination inside Docker
- `.env.compose.example`: example environment file for the compose stack
- `bootstrap-docker-compose.sh`: one-shot compose bootstrap that generates secrets and runs `docker compose up -d`
- `.env.compose.letsencrypt.example`: example env file for the Let's Encrypt compose stack
- `bootstrap-docker-compose-letsencrypt.sh`: one-shot compose bootstrap for Dockerized nginx + certbot

Runtime application configuration remains in:

- `/opt/tokilake/config/config.yaml` on the target host for the Docker bootstrap flow
- [dist/config.yaml](../dist/config.yaml) in this repository
- `deploy/runtime/config/config.yaml` for the compose bootstrap flow

Before use, replace the placeholder values in these files:

- `api.example.com`
- Linux user/group names
- installation paths such as `/opt/tokilake`
- certificate paths

Recommended production shape:

1. Run Tokilake on `127.0.0.1` or host-local port `19981`
2. Terminate HTTPS at `nginx` or another reverse proxy
3. Keep port `19981` closed from the public internet via firewall or security group

Real IP notes:

- HTTP reverse proxy: set `trusted_proxies` to your nginx/LB source IP range, then Tokilake will trust `X-Forwarded-For` / `X-Real-IP`
- TCP or `stream` proxy: set `proxy_protocol_enabled: true` and enable `proxy_protocol on;` on the upstream hop
- `trusted_proxies: "none"` means "trust no proxy headers"

One-shot bootstrap example:

```bash
sudo ./deploy/bootstrap-nginx-letsencrypt.sh \
  --domain api.example.com \
  --email admin@example.com \
  --image ghcr.io/tokimorphling/tokilake:latest \
  --config ./deploy/config.production-nginx.yaml
```

Before running it:

- make sure your DNS A/AAAA record already points to this server
- keep ports `80` and `443` reachable from the public internet during certificate issuance
- the script will persist `USER_TOKEN_SECRET` and `SESSION_SECRET` into the host config file on first run if placeholders are still present
- on first bootstrap, if `hashids_salt` is empty, the script will generate a valid random sqids alphabet and persist it; reruns preserve the existing value
- if you set `hashids_salt` manually, it must be ASCII, unique characters only, length >= 3
- if `docker` already exists on the host, the bootstrap script will not install `docker.io` again
- when using `--skip-package-install`, `docker`, `nginx`, `certbot`, `systemctl`, and `python3` must already be installed
- the bootstrap script supports both Debian-style `sites-available/sites-enabled` and `conf.d` nginx layouts

Compose bootstrap example:

```bash
./deploy/bootstrap-docker-compose.sh \
  --domain localhost \
  --scheme http \
  --http-port 8080
```

This will:

- create `deploy/runtime/` for config and data
- generate `USER_TOKEN_SECRET`, `SESSION_SECRET`, a valid random sqids alphabet, and a Postgres password if placeholders are still present
- write compose env values to `deploy/.env.compose`
- start `postgres`, `redis`, `tokilake`, and `nginx`
- run `nginx` inside Docker as part of the compose stack, not on the host

Direct compose usage:

```bash
mkdir -p ./deploy/runtime/config
cp ./deploy/.env.compose.example ./deploy/.env.compose
cp ./deploy/config.compose-nginx.yaml ./deploy/runtime/config/config.yaml
docker compose --env-file ./deploy/.env.compose -f ./deploy/docker-compose.nginx.yaml up -d
```

Notes for the compose stack:

- it publishes nginx on HTTP only by default
- `tokilake` is not published directly; nginx talks to it over the internal compose network
- if you later add TLS in front of nginx, update `server_address` in `deploy/runtime/config/config.yaml` to the final external `https://...` address

Compose + Let's Encrypt bootstrap example:

```bash
./deploy/bootstrap-docker-compose-letsencrypt.sh \
  --domain api.example.com \
  --email admin@example.com
```

This TLS stack:

- runs `nginx` inside Docker and publishes both `80` and `443`
- uses a Dockerized `certbot` container for initial HTTP-01 issuance
- starts a long-running `certbot-renew` container for periodic renewal
- keeps certificates in `deploy/runtime-letsencrypt/letsencrypt`
- reloads `nginx` periodically so renewed certificates are picked up without manual intervention

Before using the Let's Encrypt stack:

- make sure `api.example.com` already resolves to this machine
- keep ports `80` and `443` reachable from the public internet
- use `--staging` first if you want to validate the flow without consuming production rate limits
