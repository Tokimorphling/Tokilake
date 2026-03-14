# Deploy Templates

This directory contains production deployment templates for Tokilake.

- `tokilake.service`: `systemd` unit template for running the backend on port `19981`
- `nginx.tokilake.conf`: `nginx` reverse proxy template with TLS termination, WebSocket upgrade, and streaming-friendly defaults
- `nginx.stream.tokilake.conf`: `nginx stream` TCP proxy example with PROXY protocol
- `config.local-test.yaml`: local single-machine template with direct access and proxy trust disabled
- `config.production-nginx.yaml`: production template for Tokilake behind nginx on port `19981`
- `bootstrap-nginx-letsencrypt.sh`: one-shot bootstrap script for Debian/Ubuntu, Docker, nginx, and Let's Encrypt

Runtime application configuration remains in:

- `/opt/tokilake/config/config.yaml` on the target host for the Docker bootstrap flow
- [dist/config.yaml](/Users/asuka/codes/Tokilake/dist/config.yaml) in this repository

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
- the script will persist `USER_TOKEN_SECRET`, `SESSION_SECRET`, and `hashids_salt` into the host config file on first run if placeholders are still present
