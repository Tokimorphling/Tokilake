# 通过 Chat Completions 生成图像（OpenAI SDK）

<p align="right">
  <a href="./ImageGenChat.md">English</a> | <strong>中文</strong>
</p>

本教程演示如何将**扩散模型图像生成后端**（如 `sglang-diffusion`、`vllm-omni` 或任何 OpenAI 兼容的图像模型）接入 Tokilake 网关，并通过 **标准 OpenAI Python SDK** 从任意位置调用 —— 使用的是大家熟悉的 `/v1/chat/completions` 端点。

与 [图像生成指南](./ImageGen.zh.md) 使用专用 `/v1/images/generations` 端点不同，本方案将图像模型视为普通聊天模型。扩散模型特有的参数（`height`、`width`、`num_inference_steps`、`seed` 等）通过 `extra_body` 传递，Tokilake 会将其透明转发至后端。

> **前置条件：** 你需要已部署好的 Tokilake 网关，以及已注册的账号、私有分组和令牌。如尚未完成，请先阅读 [使用指南](./guide.zh.md)。

---

## 架构概览

```
┌──────────────────────┐
│   你的 Python 应用    │
│   (OpenAI SDK)       │
│                      │
│  client.chat.comple- │
│  tions.create(       │
│    model="Z-Image",  │
│    messages=[...],   │
│    extra_body={...}  │
│  )                   │
└─────────┬────────────┘
          │  POST /v1/chat/completions
          ▼
┌──────────────────────┐
│  Tokilake 网关       │
│                      │
│  • 鉴权 (token)      │
│  • 路由到渠道         │
│  • 转发 extra_body   │
└─────────┬────────────┘
          │  (WebSocket/QUIC 隧道)
          ▼
┌──────────────────────┐
│  Tokiame (工作节点)   │
│                      │
│  • 反向隧道           │
│  • 模型名映射         │
└─────────┬────────────┘
          │  POST /v1/chat/completions
          ▼
┌──────────────────────┐
│  图像生成后端         │
│  (SGLang / vLLM /    │
│   ComfyUI 等)        │
│                      │
│  本地 GPU 上的        │
│  扩散模型             │
└──────────────────────┘
```

---

## 1. 启动图像生成后端

你需要一个暴露 **OpenAI 兼容 `/v1/chat/completions`** 端点的扩散模型服务器。常见选项：

| 后端 | 说明 |
|------|------|
| **SGLang** (`sglang serve`) | 高性能推理服务，支持 RadixAttention，兼容多种扩散模型 |
| **vLLM-Omni** | vLLM 的多模态生成分支 |
| **ComfyUI** + OpenAI 包装层 | 灵活的节点式工作流，搭配 OpenAI 兼容 API 层 |

### 示例：SGLang

```bash
sglang serve \
  --model-path Wan-AI/Wan2.1-T2V-14B \
  --host 0.0.0.0 \
  --port 8122
```

### 示例：Ollama（支持图像生成的多模态 LLM）

```bash
ollama run llava
```

> **注意：** 具体模型和参数取决于你的后端。核心要求是它能通过 `messages` 接受 prompt，并在响应中返回图像数据。

---

## 2. 本地测试后端

在接入 Tokilake 之前，先验证后端是否正常工作。

```python
from openai import OpenAI
import base64

client = OpenAI(base_url="http://127.0.0.1:8122/v1", api_key="x")

prompt = (
    "Tilt POV shot of a hand holding a surreal popsicle with a transparent "
    "blue exterior, revealing an underwater scene inside: a tiny scuba diver "
    "with tiny fish floating with bubbles, ocean waves crashing, and a green "
    "popsicle stick running through the center. The popsicle is melting slightly, "
    "with a wooden stick at the bottom, hand is holding it by the wooden stick, "
    "soft focus new york street background, premium product photography"
)

print("正在生成图像...")
response = client.chat.completions.create(
    model="your-model-name",
    messages=[{"role": "user", "content": prompt}],
    extra_body={
        "height": 1024,
        "width": 1024,
        "num_inference_steps": 50,
        "true_cfg_scale": 4.0,
        "seed": 42,
    },
)

# 提取并保存图像
message_content = response.choices[0].message.content
if isinstance(message_content, list):
    img_url = message_content[0].get("image_url", {}).get("url", "")
elif isinstance(message_content, str):
    img_url = message_content

if img_url and img_url.startswith("data:image"):
    _, b64_data = img_url.split(",", 1)
    with open("output.png", "wb") as f:
        f.write(base64.b64decode(b64_data))
    print("已保存: output.png")
else:
    print("响应:", message_content[:200])
```

如果几秒内看到 `output.png`，说明后端正常工作。

---

## 3. 通过 Tokiame 将后端接入 Tokilake

### 3.1 安装 Tokiame

从当前源码目录安装：

```bash
go install ./cmd/tokiame
```

如果使用已发布版本，也可以通过 npm 安装：

```bash
npm install -g @tokilake/tokiame
```

### 3.2 配置 Tokiame

编辑配置文件 `~/.tokilake/tokiame.json`：

```json
{
  "gateway_url": "wss://YOUR_TOKILAKE_HOST/api/tokilake/connect",
  "token": "YOUR_TOKILAKE_TOKEN",
  "namespace": "gpu-01",
  "node_name": "node-1",
  "group": "YOUR_GROUP_NAME",
  "backend_type": "openai",
  "heartbeat_interval_seconds": 15,
  "reconnect_delay_seconds": 5,
  "model_targets": {
    "Z-Image-Turbo": {
      "mapped_name": "your-backend-model-name",
      "url": "http://127.0.0.1:8122/v1",
      "api_keys": ["x"],
      "price": {}
    }
  }
}
```

**关键字段说明：**

| 字段 | 说明 |
|------|------|
| `model_targets` 的 key（如 `Z-Image-Turbo`） | 通过 Tokilake 调用时使用的模型名 |
| `mapped_name` | 后端真实模型名（会作为 `model` 字段透传给后端） |
| `url` | 本地后端的 base URL |
| `api_keys` | 后端的 API Key（如无需鉴权可填 `["x"]`） |

### 3.3 启动 Tokiame

```bash
tokiame
```

连接成功后，你会看到类似日志：

```
worker connected group=... models=[Z-Image-Turbo] backend_type=openai
```

### 3.4 在渠道设置中启用 `AllowExtraBody`

这一步**至关重要** —— 只有启用后，扩散模型参数（`height`、`width`、`num_inference_steps` 等）才能透传到后端。

1. 进入 Tokilake 控制面板 → **渠道 (Channel)** 页面。
2. 找到 Tokiame 自动创建的渠道（类型为 `100`）。
3. 编辑渠道，**启用 `Allow Extra Body`（允许额外请求体）**。
4. 保存。

如不启用，Tokilake 会剥离额外参数，后端将使用默认值。

---

## 4. 从任意位置调用

现在你可以从任何有网络的机器上，使用标准 OpenAI SDK 生成图像。

```python
from openai import OpenAI
import base64

# 配置
API_KEY = "sk-YOUR_TOKILAKE_TOKEN"
BASE_URL = "https://YOUR_TOKILAKE_HOST/v1"

PROMPT = (
    "Tilt POV shot of a hand holding a surreal popsicle with a transparent "
    "blue exterior, revealing an underwater scene inside: a tiny scuba diver "
    "with tiny fish floating with bubbles, ocean waves crashing, and a green "
    "popsicle stick running through the center. The popsicle is melting slightly, "
    "with a wooden stick at the bottom, hand is holding it by the wooden stick, "
    "soft focus new york street background, premium product photography"
)

client = OpenAI(base_url=BASE_URL, api_key=API_KEY)

print("正在生成图像...")
response = client.chat.completions.create(
    model="Z-Image-Turbo",
    messages=[{"role": "user", "content": PROMPT}],
    extra_body={
        "height": 1024,
        "width": 1024,
        "num_inference_steps": 50,
        "true_cfg_scale": 4.0,
        "seed": 42,
    },
)


def extract_and_save_image(response, filename="output.png"):
    try:
        message_content = response.choices[0].message.content

        img_url = None
        if isinstance(message_content, list) and len(message_content) > 0:
            item = message_content[0]
            if isinstance(item, dict):
                img_url = item.get("image_url", {}).get("url")
            else:
                img_url = getattr(getattr(item, "image_url", None), "url", None)
        elif isinstance(message_content, str):
            img_url = message_content

        if not img_url:
            print("错误: 响应中未找到图像 URL")
            return

        if img_url.startswith("data:image"):
            _, b64_data = img_url.split(",", 1)
            with open(filename, "wb") as f:
                f.write(base64.b64decode(b64_data))
            print(f"成功: 图像已保存至 {filename}")
        else:
            print(f"找到图像 URL (非 base64): {img_url}")

    except Exception as e:
        print(f"提取图像时出错: {e}")


extract_and_save_image(response)
```

你应该能在本地看到保存的 `output.png` —— 与直接调用后端生成的图像一致。

以下是通过 Tokilake 调用运行在 **Google Colab (A100 GPU)** 上的 **Z-Image-Turbo** 生成的示例输出：

![示例输出 — Colab 上的 Z-Image-Turbo](./ImageGenChat-output.png)

---

## 常用 `extra_body` 参数

具体参数取决于你的后端和模型，以下是最常见的：

| 参数 | 类型 | 说明 |
|------|------|------|
| `height` | int | 输出图像高度（像素），如 `1024` |
| `width` | int | 输出图像宽度（像素），如 `1024` |
| `num_inference_steps` | int | 去噪步数。越高 = 质量越好，速度越慢 |
| `true_cfg_scale` | float | Classifier-free guidance scale，控制对 prompt 的遵循程度 |
| `guidance_scale` | float | CFG scale 的别名（部分后端使用此名称） |
| `seed` | int | 随机种子，用于复现结果 |
| `negative_prompt` | str | 生成图像中需要避免的内容 |

---

## 常见问题排查

| 现象 | 原因 | 解决方法 |
|------|------|---------|
| `extra_body` 参数被忽略 | 渠道未启用 `AllowExtraBody` | 在渠道设置中启用（步骤 3.4） |
| `worker connected` 但找不到模型 | 模型名不匹配 | 确保 `model_targets` 的 key 与 API 调用中的 `model` 一致 |
| 图像以 URL 而非 base64 返回 | 后端返回 URL 格式 | 代码中同时处理 `data:image` base64 和普通 URL |
| 连接超时 | Tokiame 无法访问后端 | 检查 `model_targets` 中的 `url` 在 Tokiame 运行机器上是否可达 |
| Token 鉴权失败 | Token 未绑定到正确的分组 | 验证 Token 是否关联到 Tokiame 工作节点加入的分组 |

---

## 相关文档

- [使用指南 (中文)](./guide.zh.md) — 完整部署流程
- [User Guide (English)](./guide.en.md) — 英文部署指南
- [图像生成指南](./ImageGen.zh.md) — 使用专用 `/v1/images/generations` 端点
- [Image Generation Guide](./ImageGen.md) — Using the dedicated `/v1/images/generations` endpoint
