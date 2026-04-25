---
title: "升级"
layout: doc
outline: deep
lastUpdated: true
---

# 升级

## 生产部署更新

如果你使用推荐的 `deploy/bootstrap-nginx-letsencrypt.sh` 部署，更新镜像时继续使用同一个脚本：

```bash
cd /path/to/Tokilake

sudo ./deploy/bootstrap-nginx-letsencrypt.sh \
  --domain api.example.com \
  --update
```

脚本会复用已有配置和证书，默认拉取最新镜像，并重建容器，同时保留 nginx HTTPS / WebSocket / QUIC 配置。

如果只想用本地已有镜像重启容器：

```bash
sudo ./deploy/bootstrap-nginx-letsencrypt.sh \
  --domain api.example.com \
  --update \
  --no-pull
```

## 定时更新

可以用 crontab 定期执行更新脚本。例如每天凌晨 4 点更新：

```cron
0 4 * * * cd /path/to/Tokilake && sudo ./deploy/bootstrap-nginx-letsencrypt.sh --domain api.example.com --update >> /var/log/tokilake-update.log 2>&1
```

::: warning 注意
首次申请证书时必须传 `--email`。已有证书后，`--update` 可以只传 `--domain`。
:::

## 本地 Docker 示例更新

如果你使用的是文档里的最小本地 Docker 示例，可以手动拉取镜像并重建容器：

```bash
docker pull ghcr.io/tokimorphling/tokilake:latest
docker rm -f tokilake-local

docker run -d \
  --name tokilake-local \
  --restart unless-stopped \
  -p 19981:19981 \
  -e TZ=Asia/Shanghai \
  -e PORT=19981 \
  -e GIN_MODE=release \
  -e SERVER_ADDRESS="http://localhost:19981" \
  -e USER_TOKEN_SECRET="replace-with-your-existing-user-token-secret" \
  -e SESSION_SECRET="replace-with-your-existing-session-secret" \
  -v tokilake-local-data:/data \
  ghcr.io/tokimorphling/tokilake:latest
```

本地重建容器时请复用原来的 `USER_TOKEN_SECRET`，否则已有用户令牌会失效。
