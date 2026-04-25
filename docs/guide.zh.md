# Tokilake 使用指南

Tokilake 是一个去中心化的大模型 API 调度网关，允许位于 NAT/内网之后的 GPU 工作节点（Tokiame）连接到中心网关（Hub）。

## 1. 部署 Tokilake (Hub)

如果你只想作为客户端（Tokiame）运行，可以跳过此步骤。

### 生产环境部署

如果要部署一个公开可访问的生产 Hub，并自动配置 HTTPS、Let's Encrypt、nginx、Docker 和 QUIC，推荐使用部署脚本：

```bash
git clone https://github.com/Tokimorphling/Tokilake.git
cd Tokilake

sudo ./deploy/bootstrap-nginx-letsencrypt.sh \
  --domain api.example.com \
  --email admin@example.com \
  --sql-dsn 'postgres://user:password@127.0.0.1:5432/tokilake'
```

后续更新镜像：

```bash
sudo ./deploy/bootstrap-nginx-letsencrypt.sh \
  --domain api.example.com \
  --update
```

### 最小本地部署

如果只想在本机快速试跑，不需要 nginx 和证书：

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

容器启动后访问 `http://localhost:19981`。

## 2. 注册与登录

1. 访问 Tokilake 控制面板（使用上面的本地 Docker 示例时为 `http://localhost:19981`）。
2. 在控制面板中，你会看到 **私有分组 (Private Groups)**，这是 Tokilake 的核心机制。
3. 点击 **创建分组 (Create Group)** 按钮创建一个新的私有分组。
4. 在 **Actions/Manage Group** 页面，你可以管理成员和邀请。
5. 如果你有闲置的 GPU 资源并希望分享，可以点击 **生成邀请码 (Generate invite code)**。
6. 在 **令牌 (Token)** 页面创建一个令牌，并将其绑定到你创建的私有分组。**请记录下这个令牌。**

## 3. 部署 Tokiame (Worker)

即便你的 GPU 位于没有公网 IP 的 NAT 网络、住宅网络或 Google Colab 中，你依然可以部署 Tokiame。

### 3.1 启动推理服务

以 `llama.cpp` 为例启动推理服务：

```bash
export LLAMA_CACHE="unsloth/Qwen3.5-9B-GGUF"
./llama-server \
    -hf unsloth/Qwen3.5-9B-GGUF:UD-Q4_K_XL \
    --ctx-size 16384 \
    --temp 1.0 \
    --top-p 0.95 \
    --top-k 20 \
    --min-p 0.00 \
    --alias "unsloth/Qwen3.5-9B-GGUF" \
    --port 8001 \
    --chat-template-kwargs '{"enable_thinking":true}'
```

### 3.2 安装并配置 Tokiame

从当前源码目录安装 Tokiame：

```bash
go install ./cmd/tokiame
```

如果你使用的是已发布版本，也可以通过 npm 安装器安装：

```bash
npm install -g @tokilake/tokiame
```

修改配置文件 `~/.tokilake/tokiame.json`：

```json
{
    "gateway_url": "wss://YOUR_TOKILAKE_IP/api/tokilake/connect",
    "token": "你的_TOKEN",
    "namespace": "gpu-01",
    "node_name": "node-1",
    "group": "你的_GROUP_NAME",
    "backend_type": "openai",
    "heartbeat_interval_seconds": 15,
    "reconnect_delay_seconds": 5,
    "model_targets": {
        "unsloth_Qwen3.5-9B-GGUF_Qwen3.5-9B-UD-Q4_K_XL": {
            "mapped_name": "unsloth/Qwen3.5-9B-GGUF",
            "url": "http://127.0.0.1:8001/v1",
            "api_keys": ["x"],
            "price": {}
        }
    }
}
```

### 3.3 启动 Tokiame

运行 `tokiame` 客户端。成功连接后，你会在日志中看到：
`worker connected group=... models=[...]`

![alt text](image.png)

## 4. 分享 GPU 资源

1. 让你的朋友在 Tokilake 注册。
2. 在 **私有分组** 页面，让他们使用 **使用邀请码 (Redeem Invite Code)** 功能。
3. 加入分组后，他们就能看到并使用你分享的模型。
4. 他们需要创建一个 **API Key** 并绑定到该分组。
5. 可以在控制面的 **Actions/Chat** 中直接测试模型。

![alt text](image-1.png)
**祝你使用愉快！**
