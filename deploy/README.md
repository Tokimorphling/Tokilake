# Deploy

`deploy/` 现在只保留一条生产部署路径：

- 宿主机 `nginx` 负责 HTTPS 终止和 HTTP/WebSocket 反代
- Let's Encrypt 负责签发和续期证书
- Tokilake 运行在 Docker 容器里
- `nginx stream` 把 `443/udp` 转发给 Tokilake QUIC

## 首次部署

```bash
sudo ./deploy/bootstrap-nginx-letsencrypt.sh \
  --domain api.example.com \
  --email admin@example.com \
  --sql-dsn 'postgres://user:password@127.0.0.1:5432/tokilake'
```

如果不传 `--sql-dsn`，Tokilake 会使用 SQLite，数据库文件在 `/opt/tokilake/data/one-api.db`。

常用参数：

```bash
--image ghcr.io/tokimorphling/tokilake:latest
--app-dir /opt/tokilake
--container-name tokilake
--port 19981
--redis redis://127.0.0.1:6379/0
--skip-package-install
```

## 更新镜像

重新执行同一个脚本并加上 `--update`。脚本会复用已有配置和证书，默认拉取最新镜像，然后按同样的 nginx/QUIC 配置重建容器。

```bash
sudo ./deploy/bootstrap-nginx-letsencrypt.sh \
  --domain api.example.com \
  --update
```

如果只想用本地已有镜像重启容器，可以加 `--no-pull`。

## 网络

宿主机防火墙和云安全组需要放行：

- `80/tcp`：Let's Encrypt HTTP challenge
- `443/tcp`：HTTPS 和 WebSocket
- `443/udp`：QUIC

生成后的应用配置在 `/opt/tokilake/config/config.yaml`，数据和日志在 `/opt/tokilake/data`。
