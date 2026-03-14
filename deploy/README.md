# Deploy Templates

This directory contains production deployment templates for Tokilake.

- `tokilake.service`: `systemd` unit template for running the backend on port `19981`
- `nginx.tokilake.conf`: `nginx` reverse proxy template with TLS termination, WebSocket upgrade, and streaming-friendly defaults
- `nginx.stream.tokilake.conf`: `nginx stream` TCP proxy example with PROXY protocol
- `config.local-test.yaml`: local single-machine template with direct access and proxy trust disabled
- `config.production-nginx.yaml`: production template for Tokilake behind nginx on port `19981`

Runtime application configuration remains in:

- `/opt/tokilake/dist/config.yaml` on the target host
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
