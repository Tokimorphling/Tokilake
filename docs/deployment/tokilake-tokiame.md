---
title: "Tokilake 与 Tokiame"
layout: doc
outline: deep
lastUpdated: true
---

# Tokilake 与 Tokiame

本页说明如何在 `master` 分支上启用 Tokilake 网关与 Tokiame worker。

## 能力范围

- Tokiame 通过 `GET /api/tokilake/connect` 连接网关。
- 连接成功后会自动创建或更新一个 `Tokiame/<namespace>` 渠道。
- 支持的标准接口包括：
  - `/v1/chat/completions`
  - `/v1/completions`
  - `/v1/embeddings`
  - `/v1/audio/speech`
  - `/v1/audio/transcriptions`
  - `/v1/audio/translations`
  - `/v1/images/generations`
  - `/v1/images/edits`
  - `/v1/images/variations`
  - `/v1/responses`
  - `/v1/rerank`
- 支持的视频异步任务接口包括：
  - `POST /v1/videos`
  - `GET /v1/videos`
  - `GET /v1/videos/:id`
  - `GET /v1/videos/:id/content`

## 前置条件

1. 先启动 Tokilake 主服务。
2. 准备一个普通用户令牌或管理员为某个用户创建的令牌，Tokiame 将使用这个令牌连接 Tokilake。
3. 准备本地模型服务或兼容 OpenAI 的上游地址。

## 启动网关

Tokilake 网关已经集成在主服务中，正常启动主程序即可：

```bash
go run .
```

或使用你现有的部署方式启动 Tokilake。

如果你使用容器方式运行主服务：

```bash
docker run -d -p 19981:19981 \
  --name tokilake \
  --restart always \
  -e PORT=19981 \
  -e SERVER_ADDRESS="https://api.example.com" \
  -e USER_TOKEN_SECRET="user_token_secret" \
  -e SESSION_SECRET="session_secret" \
  -v $(pwd)/data:/data \
  ghcr.io/<your-org>/tokilake:latest
```

## 启动 Tokiame

Tokiame 作为独立 worker 运行：

```bash
go run ./cmd/tokiame
```

最小环境变量示例：

```bash
export TOKIAME_GATEWAY_URL="ws://127.0.0.1:3000/api/tokilake/connect"
export TOKIAME_TOKEN="sk-your-user-token"
export TOKIAME_NAMESPACE="demo-worker"
export TOKIAME_GROUP="demo-group"
export TOKIAME_BACKEND_TYPE="openai"
export TOKIAME_MODEL_TARGETS='{
  "gpt-4o-mini": {
    "url": "http://127.0.0.1:8000/v1",
    "mapped_name": "gpt-4o-mini"
  },
  "text-embedding-3-small": {
    "url": "http://127.0.0.1:8000/v1"
  }
}'

go run ./cmd/tokiame
```

也可以直接指定 JSON 配置文件：

```bash
go run ./cmd/tokiame --config ./packaging/tokiame.json.example
```

也可以直接使用容器镜像运行：

```bash
docker run --rm \
  --name tokiame \
  -e TOKIAME_GATEWAY_URL="wss://api.example.com/api/tokilake/connect" \
  -e TOKIAME_TOKEN="sk-your-user-token" \
  -e TOKIAME_NAMESPACE="demo-worker" \
  -e TOKIAME_GROUP="demo-group" \
  -e TOKIAME_MODEL_TARGETS='{
    "gpt-4o-mini": {
      "url": "http://127.0.0.1:8000/v1"
    }
  }' \
  ghcr.io/<your-org>/tokiame:latest
```

如果你有 JSON 配置文件，也可以挂载后通过参数传入：

```bash
docker run --rm \
  --name tokiame \
  -v $(pwd)/tokiame.json:/data/tokiame.json:ro \
  ghcr.io/<your-org>/tokiame:latest \
  --config /data/tokiame.json
```

## 环境变量说明

- `TOKIAME_GATEWAY_URL`: Tokilake websocket 地址。
- `TOKIAME_TOKEN`: 用于连接网关的用户令牌。
- `TOKIAME_NAMESPACE`: worker 的唯一命名空间，同一时刻不能重复；一旦首次注册成功，后续只允许同一用户重新接管。
- `TOKIAME_NODE_NAME`: 可选，节点显示名。
- `TOKIAME_GROUP`: 可选，渠道所属分组；只能使用当前用户的主用户分组，或当前用户已拥有/已加入的私有分组。省略时默认使用当前用户的主用户分组。
- `TOKIAME_BACKEND_TYPE`: 可选，默认 `openai`。
- `TOKIAME_MODEL_TARGETS`: 必填，JSON 格式的模型到本地目标映射。
- `TOKIAME_HEARTBEAT_INTERVAL_SECONDS`: 可选，心跳间隔。
- `TOKIAME_RECONNECT_DELAY_SECONDS`: 可选，断线重连间隔。
- `TOKIAME_CONFIG`: 可选，JSON 配置文件路径；环境变量优先级更高。
- `--config` / `-c`: 可选，等价于设置 `TOKIAME_CONFIG`。
- 如果 `--config` 和 `TOKIAME_CONFIG` 都未设置，`tokiame` 会自动尝试加载 `~/.tokilake/tokiame.json`。

`TOKIAME_MODEL_TARGETS` 支持的字段：

- `url`: 本地或上游服务地址，必须包含协议和主机。
- `mapped_name`: 可选，将外部模型名重写为本地模型名。
- `backend_type`: 可选，覆盖默认后端类型。
- `headers`: 可选，附加请求头。
- `api_keys`: 可选，目标服务 API Key 列表，多个 key 会轮询使用。
- `api_key_header`: 可选，默认 `Authorization`。
- `api_key_prefix`: 可选，默认 `Bearer `。

示例：

```json
{
  "deepseek-chat": {
    "url": "http://127.0.0.1:8000/v1",
    "mapped_name": "deepseek-chat",
    "headers": {
      "X-Source": "tokiame"
    }
  }
}
```

## 自动建渠道行为

首次连接成功后，Tokilake 会自动：

1. 记录 `tokilake_worker_nodes` 节点信息。
2. 创建或更新一个 `type = 100` 的 `Tokiame/<namespace>` 渠道。
3. 用 `TOKIAME_GROUP` 和 `TOKIAME_MODEL_TARGETS` 同步渠道的分组与模型。
4. 刷新渠道缓存，使新模型立即参与分发。

## 分组行为

- `TOKIAME_GROUP` 必须是当前用户有权使用的分组；私有分组需要先创建并授予当前用户访问权限。
- `TOKIAME_GROUP` 对应的 `user_groups` 记录如果不存在，会在授权校验通过后自动创建。
- 自动创建的分组默认为非公开分组。
- 如果希望其他用户也能直接在令牌中选择这个分组，需要管理员把对应 `user_group` 调整为公开分组，或把这些用户的用户分组改成同名分组。

## 调用示例

当 worker 注册成功、渠道在线后，普通 API 调用方式保持不变：

```bash
curl http://127.0.0.1:3000/v1/chat/completions \
  -H "Authorization: Bearer sk-user-api-token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {"role": "user", "content": "hello"}
    ]
  }'
```

只要当前请求命中的分组里存在这个模型，Tokilake 就会把请求隧道转发给对应的 Tokiame worker。

## 视频任务调用示例

视频任务接口统一使用 `/v1/videos`，当前支持 `text2video` 和 `image2video`：

```bash
curl http://127.0.0.1:3000/v1/videos \
  -H "Authorization: Bearer sk-user-api-token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "wanx2.1-t2v",
    "mode": "text2video",
    "prompt": "A red sports car driving along a coast road at sunset",
    "size": "1280x720",
    "duration": 5,
    "fps": 24
  }'
```

返回结果是一个异步任务对象：

```json
{
  "id": "vid_123",
  "object": "video",
  "created": 1710000000,
  "model": "wanx2.1-t2v",
  "mode": "text2video",
  "status": "queued",
  "content_url": "/v1/videos/vid_123/content",
  "download_url": "/v1/videos/vid_123/content"
}
```

随后可以轮询任务状态：

```bash
curl http://127.0.0.1:3000/v1/videos/vid_123 \
  -H "Authorization: Bearer sk-user-api-token"
```

或列出当前用户最近的视频任务：

```bash
curl "http://127.0.0.1:3000/v1/videos?limit=20&status=processing" \
  -H "Authorization: Bearer sk-user-api-token"
```

任务完成后可直接下载内容：

```bash
curl http://127.0.0.1:3000/v1/videos/vid_123/content \
  -H "Authorization: Bearer sk-user-api-token" \
  --output result.mp4
```

下载接口的返回语义如下：

- 任务未完成时返回 `409 video_not_ready`
- 任务失败时返回 `502 video_failed`
- 任务成功时由 Tokilake 通过 Tokiame 隧道代理视频二进制内容

Tokiame 本地后端需要实现以下兼容接口：

- `POST /v1/videos`
- `GET /v1/videos/{id}`
- `GET /v1/videos/{id}/content`

## 常见问题

## 分发建议

当前更推荐的 Tokiame 分发方式按优先级排序如下：

1. GitHub Releases 预编译二进制
2. `npm` 薄包装安装器
3. Homebrew
4. OCI 镜像，例如 `ghcr.io/<your-org>/tokiame:latest`

这里的关键设计是：**GitHub Releases 是真实二进制源，npm 和 Homebrew 只是安装壳。**

从当前仓库的 release workflow 开始，Tokiame 会随版本标签 `vX.Y.Z` 一起产出：

- `tokiame_<version>_linux_amd64.tar.gz`
- `tokiame_<version>_linux_arm64.tar.gz`
- `tokiame_<version>_darwin_amd64.tar.gz`
- `tokiame_<version>_darwin_arm64.tar.gz`
- `tokiame_<version>_windows_amd64.zip`
- `SHA256SUMS-linux.txt`
- `SHA256SUMS-darwin.txt`
- `SHA256SUMS-windows.txt`

### npm 安装

仓库内置了一个很薄的 npm 包骨架：`packaging/npm/tokiame`。它的行为是：

- `npm install -g @tokilake/tokiame`
- `postinstall` 根据当前平台下载对应 GitHub Release 归档
- 校验 `SHA256SUMS-<os>.txt`
- 把 `tokiame` 暴露成全局命令
- 首次安装时自动写入 `~/.tokilake/tokiame.json.example`

如果你把 GitHub Release 标记为正式发布，仓库里的 `Publish Tokiame npm` workflow 也可以顺手把 `@tokilake/tokiame` 发布到 npm。

运行时默认配置路径仍然是：

```text
~/.tokilake/tokiame.json
```

安装后建议：

```bash
cp ~/.tokilake/tokiame.json.example ~/.tokilake/tokiame.json
tokiame
```

### Homebrew 安装

仓库内置了 formula 渲染脚本：

```bash
./hack/scripts/render-tokiame-homebrew-formula.sh \
  --version 1.2.3 \
  --darwin-amd64 <sha256> \
  --darwin-arm64 <sha256> \
  --linux-amd64 <sha256> \
  --linux-arm64 <sha256> \
  > packaging/homebrew/tokiame.rb
```

推荐做法是把生成后的 `tokiame.rb` 提交到单独的 Homebrew tap 仓库。公式安装后的默认配置路径同样是：

```text
~/.tokilake/tokiame.json
```

### 容器安装

如果你希望在服务器侧用容器分发，仍然建议把 `tokiame` 当作无状态 worker：

- 镜像里只放二进制和 `tokiame.json.example`
- 真实配置通过环境变量或挂载 `/root/.tokilake/tokiame.json` 注入
- worker 本身不持久化业务状态

### 1. `namespace already connected`

说明同一个 `TOKIAME_NAMESPACE` 已经有在线 worker。请更换命名空间，或等待旧连接断开。

### 1.1 `namespace_not_owned`

说明这个 `TOKIAME_NAMESPACE` 已经被其他用户首次注册并持有。请更换命名空间，或使用原用户的令牌重新连接。

### 2. worker 在线，但模型没有出现

优先检查：

- `TOKIAME_MODEL_TARGETS` 是否是合法 JSON。
- `TOKIAME_GROUP` 是否和预期一致。
- 当前连接用户是否真的拥有该分组，或已经加入对应私有分组。
- 对应渠道是否已自动创建成功。
- 当前用户是否有权限使用该分组。

### 3. 请求报 `tokiame session is offline`

说明渠道记录仍在，但对应 worker 已离线。需要确认：

- Tokiame 进程是否还在运行。
- websocket 地址与令牌是否正确。
- worker 是否因为本地上游不可用而反复重连。
