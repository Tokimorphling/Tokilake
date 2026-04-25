---
title: "部署说明"
layout: doc
outline: deep
lastUpdated: true
---

# 部署说明

## 配置说明

系统支持两种配置方式：

1. 环境变量
2. 配置文件 (config.yaml)

::: tip 配置优先级
环境变量 > 配置文件
:::

## 扩展组件

- [Tokilake 与 Tokiame](./tokilake-tokiame.md): 使用内置网关接入自托管模型 worker。

### 必要配置

- `USER_TOKEN_SECRET`: 必填，用于生成用户令牌的密钥
- `SESSION_SECRET`: 推荐填写，用于保持用户登录状态，如果不设置，每次重启后已登录用户需要重新登录

推荐做法：

- `USER_TOKEN_SECRET` 使用一次生成、长期固定保存的强随机字符串
- 不要在每次容器启动时重新生成，否则已有用户 token 会全部失效
- 可用下面的命令生成：

```bash
openssl rand -hex 32
```

## 生产部署（推荐）

当前仓库只保留一条推荐生产部署路径：

- 宿主机运行 `nginx`
- Tokilake 运行在 Docker 容器里
- Let's Encrypt 自动签发证书
- `443/tcp` 提供 HTTPS / WebSocket
- `443/udp` 通过 `nginx stream` 转发给 Tokilake QUIC

前置条件：

- DNS 已经指向当前服务器
- 防火墙和云安全组已放行 `80/tcp`、`443/tcp`、`443/udp`
- 服务器是 Debian/Ubuntu 风格系统，能够使用 `apt`

首次部署：

```bash
git clone https://github.com/Tokimorphling/Tokilake.git
cd Tokilake

sudo ./deploy/bootstrap-nginx-letsencrypt.sh \
  --domain api.example.com \
  --email admin@example.com \
  --sql-dsn 'postgres://user:password@127.0.0.1:5432/tokilake'
```

如果不传 `--sql-dsn`，Tokilake 会使用 SQLite，数据库文件保存在 `/opt/tokilake/data/one-api.db`。生产环境建议使用 PostgreSQL 或 MySQL。

后续更新镜像：

```bash
sudo ./deploy/bootstrap-nginx-letsencrypt.sh \
  --domain api.example.com \
  --update
```

常用参数：

```bash
--image ghcr.io/tokimorphling/tokilake:latest
--app-dir /opt/tokilake
--container-name tokilake
--port 19981
--redis redis://127.0.0.1:6379/0
--no-pull
```

生成后的应用配置位于 `/opt/tokilake/config/config.yaml`，数据和日志位于 `/opt/tokilake/data`。

## 最小本地部署

如果只想在本机快速试跑，不需要 nginx、域名或证书：

```bash
docker run -d \
  --name tokilake-local \
  --restart unless-stopped \
  -p 19981:19981 \
  -e TZ=Asia/Shanghai \
  -e PORT=19981 \
  -e GIN_MODE=release \
  -e SERVER_ADDRESS="http://localhost:19981" \
  -e USER_TOKEN_SECRET="$(openssl rand -hex 32)" \
  -e SESSION_SECRET="$(openssl rand -hex 32)" \
  -v tokilake-local-data:/data \
  ghcr.io/tokimorphling/tokilake:latest
```

启动后访问：

```text
http://localhost:19981
```

本地示例使用 SQLite，数据库和日志保存在 Docker volume `tokilake-local-data` 中。

## 自定义 Docker 部署

如果你不使用生产脚本，也可以直接运行镜像。容器默认会尝试读取 `/data/config.yaml`；如果该文件不存在，则使用环境变量和默认值。

更多环境变量说明请参考 [环境变量](./env.md)。

SQLite 示例：

```bash
docker run -d \
  --name tokilake \
  --restart always \
  -p 19981:19981 \
  -e TZ=Asia/Shanghai \
  -e PORT=19981 \
  -e GIN_MODE=release \
  -e SERVER_ADDRESS="https://api.example.com" \
  -e USER_TOKEN_SECRET="replace-with-a-stable-random-secret" \
  -e SESSION_SECRET="replace-with-a-stable-random-secret" \
  -v /opt/tokilake/data:/data \
  ghcr.io/tokimorphling/tokilake:latest
```

PostgreSQL 示例：

```bash
docker run -d \
  --name tokilake \
  --restart always \
  -p 19981:19981 \
  -e TZ=Asia/Shanghai \
  -e PORT=19981 \
  -e GIN_MODE=release \
  -e SERVER_ADDRESS="https://api.example.com" \
  -e USER_TOKEN_SECRET="replace-with-a-stable-random-secret" \
  -e SESSION_SECRET="replace-with-a-stable-random-secret" \
  -e SQL_DSN="postgres://user:password@db.example.com:5432/tokilake" \
  -v /opt/tokilake/data:/data \
  ghcr.io/tokimorphling/tokilake:latest
```

配置文件示例：

```bash
mkdir -p /opt/tokilake/config /opt/tokilake/data
cp ./deploy/config.production-nginx.yaml /opt/tokilake/config/config.yaml

docker run -d \
  --name tokilake \
  --restart always \
  -p 19981:19981 \
  -v /opt/tokilake/data:/data \
  -v /opt/tokilake/config/config.yaml:/data/config.yaml:ro \
  ghcr.io/tokimorphling/tokilake:latest
```

## 源码或二进制运行

从源码构建：

```bash
git clone https://github.com/Tokimorphling/Tokilake.git
cd Tokilake
task tokilake
./dist/tokilake --config ./config.example.yaml --port 19981 --log-dir ./logs
```

如果没有安装 `task`，也可以在已经构建好前端资源的情况下执行：

```bash
go build -o dist/tokilake .
./dist/tokilake --config ./config.example.yaml --port 19981 --log-dir ./logs
```

如果你需要运行 Tokiame worker，请参考 [Tokilake 与 Tokiame](./tokilake-tokiame.md)。

## 多机部署

### 准备工作

1. 确保所有服务器都安装了必要的组件：

- Docker 或 手动部署所需的组件
- Redis（如果需要使用缓存）
- MySQL 客户端（如果使用远程 MySQL）

2. 网络配置：

- 确保所有服务器能够访问主数据库
- 如果使用 Redis，确保可以访问 Redis 服务器
- 检查服务器间的防火墙设置

1. 所有服务器 `SESSION_SECRET` 设置一样的值。
2. 必须设置 `SQL_DSN`，使用 MySQL 数据库而非 SQLite，所有服务器连接同一个数据库。
3. 所有从服务器必须设置 `NODE_TYPE` 为 `slave`，不设置则默认为主服务器。
4. 设置 `SYNC_FREQUENCY` 后服务器将定期从数据库同步配置，在使用远程数据库的情况下，推荐设置该项并启用 Redis，无论主从。
5. 从服务器可以选择设置 `FRONTEND_BASE_URL`，以重定向页面请求到主服务器。
6. 从服务器上**分别**装好 Redis，设置好 `REDIS_CONN_STRING`，这样可以做到在缓存未过期的情况下数据库零访问，可以减少延迟。
7. 如果主服务器访问数据库延迟也比较高，则也需要启用 Redis，并设置 `SYNC_FREQUENCY`，以定期从数据库同步配置。
