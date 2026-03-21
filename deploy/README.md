# Deploy

当前目录保留了多种历史部署材料，但默认推荐只使用下面这套主路径。

## 推荐路径

适用场景：

- 单机部署
- 宿主机运行 `nginx`
- Tokilake 运行在 Docker 容器内
- `nginx` 负责 TLS 终止
- Tokiame 默认使用 `auto`，优先走 QUIC，失败回退 WebSocket

推荐只关注这几个文件：

- `bootstrap-nginx-letsencrypt.sh`
  - 首次部署入口
  - 安装/检查 `nginx`、`certbot`、`docker`
  - 初始化 `/opt/tokilake/config/config.yaml`
  - 申请 Let's Encrypt 证书
  - 启动 Tokilake 容器
  - 自动补齐 QUIC 所需的 nginx UDP 转发、容器 UDP 端口映射、证书挂载
- `bootstrap-nginx-letsencrypt-update.sh`
  - 日常更新入口
  - 重新拉取镜像并重建容器
  - 保持 QUIC/nginx 配置不丢失
- `bootstrap-nginx-letsencrypt-quic-update.sh`
  - QUIC 修复/补齐脚本
  - 通常由上面两个脚本自动调用
  - 也可以在只想修 QUIC 配置时单独执行
- `config.production-nginx.yaml`
  - 宿主机 `nginx + docker` 形态的默认配置模板

## 首次部署

```bash
sudo ./deploy/bootstrap-nginx-letsencrypt.sh \
  --domain api.example.com \
  --email admin@example.com \
  --config ./deploy/config.production-nginx.yaml
```

如果你有自定义配置文件，也可以直接替换 `--config`。

前置条件：

- DNS 已指向当前机器
- 公网已放行 `80/tcp`、`443/tcp`、`443/udp`
- 宿主机 `dpkg/apt` 状态正常
- 如果不是默认值，要显式传入：
  - `--app-dir`
  - `--container-name`
  - `--port`

部署完成后，Tokilake 会使用：

- `443/tcp` 提供 HTTPS / WebSocket
- `443/udp` 提供 QUIC
- 容器内 `19981/tcp` 和 `19981/udp` 提供后端接入

## 后续更新

```bash
sudo ./deploy/bootstrap-nginx-letsencrypt-update.sh \
  --domain api.example.com \
  --email admin@example.com
```

这个脚本现在不是“只更新容器”的旧行为了。它会同时确保：

- 镜像更新后容器仍然挂载 Let's Encrypt 证书
- 容器仍然暴露 UDP 端口
- `nginx stream` 的 `443/udp` 转发仍然存在
- Tokilake 的 QUIC 环境变量仍然存在

## QUIC 单独修复

如果你的 HTTP/HTTPS 已经正常，只是后来切到 QUIC 才发现：

- AWS Security Group 没开 `443/udp`
- `nginx` 没监听 `443/udp`
- 容器没映射 `19981/udp`
- 容器没挂载 `/etc/letsencrypt`

可以单独执行：

```bash
sudo ./deploy/bootstrap-nginx-letsencrypt-quic-update.sh \
  --domain api.example.com
```

## 推荐验证

先验证证书：

```bash
openssl s_client -connect api.example.com:443 -servername api.example.com </dev/null | \
  openssl x509 -noout -subject -issuer -ext subjectAltName
```

再验证 worker：

```bash
./tokiame -c tokiame.json
```

如果日志里出现下面这行，说明 QUIC 已经打通：

```text
worker connected transport=quic
```

## 其他文件

下面这些文件暂时保留，但不作为默认推荐路径：

- `bootstrap-docker-compose.sh`
- `bootstrap-docker-compose-letsencrypt.sh`
- `docker-compose.nginx.yaml`
- `docker-compose.nginx-letsencrypt.yaml`
- `config.compose-nginx.yaml`
- `nginx.compose.http.conf.template`
- `nginx.compose.acme.conf.template`
- `nginx.compose.https.conf.template`
- `nginx.stream.tokilake.conf`
- `tokilake.service`
- `config.local-test.yaml`

它们更适合历史兼容、实验、compose 化部署、或特定运维场景。默认部署请优先走上面的宿主机 `nginx + docker + Let's Encrypt + QUIC` 方案。
